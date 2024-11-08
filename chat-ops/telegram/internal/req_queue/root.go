package reqqueue

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/consts"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/reqparams"
	sdapi "github.com/kanootoko/stable-diffusion-telegram-bot/internal/sd_api"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/telegram"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/utils"
	"gocv.io/x/gocv"
)

type ReqType int

const (
	ReqTypeRender ReqType = iota
	ReqTypeUpscale
	ReqTypeKuka
)

type ReqQueueEntry struct {
	Type   ReqType
	Params reqparams.ReqParams
	TaskID uint64

	bot          *telegram.SDBot
	ReplyMessage *models.Message
	Message      *models.Message
}

func (e *ReqQueueEntry) checkWaitError(err error) time.Duration {
	var retryRegex = regexp.MustCompile(`{"retry_after":([0-9]+)}`)
	match := retryRegex.FindStringSubmatch(err.Error())
	if len(match) < 2 {
		return 0
	}

	retryAfter, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return time.Duration(retryAfter) * time.Second
}

func (e *ReqQueueEntry) sendReply(ctx context.Context, text string) {
	if e.ReplyMessage == nil {
		e.ReplyMessage = e.bot.SendReplyToMessage(ctx, e.Message, text)
	} else if e.ReplyMessage.Text != text {
		e.ReplyMessage.Text = text
		err := e.bot.EditMessage(ctx, e.ReplyMessage, text)
		if err != nil {
			fmt.Println("  reply edit error:", err)

			waitNeeded := e.checkWaitError(err)
			fmt.Println("  waiting", waitNeeded, "...")
			time.Sleep(waitNeeded)
		}
	}
}

func (e *ReqQueueEntry) convertImagesFromPNGToJPG(imgs [][]byte) error {
	for i := range imgs {
		p, err := png.Decode(bytes.NewReader(imgs[i]))
		if err != nil {
			fmt.Println("  png decode error:", err)
			return fmt.Errorf("png decode error: %w", err)
		}
		buf := new(bytes.Buffer)
		err = jpeg.Encode(buf, p, &jpeg.Options{Quality: 80})
		if err != nil {
			fmt.Println("  jpg decode error:", err)
			return fmt.Errorf("jpg decode error: %w", err)
		}
		imgs[i] = buf.Bytes()
	}
	return nil
}

// If filename is empty then a filename will be automatically generated.
func (e *ReqQueueEntry) uploadImages(
	ctx context.Context,
	firstImageID uint32,
	description string,
	imgs [][]byte,
	filename string,
	retryAllowed bool,
	sendPNGs bool,
) error {
	fileExt := "jpg"
	if sendPNGs {
		fileExt = "png"
	}
	if len(imgs) == 0 {
		fmt.Println("  error: nothing to upload")
		return fmt.Errorf("nothing to upload")
	}

	generateFilename := (filename == "")

	caption := description
	if len(caption) > 1024 {
		caption = caption[:1021] + "..."
	}
	var err error
	var media []models.InputMedia
	for i := range imgs {
		if generateFilename {
			filename = fmt.Sprintf("sd-image-%d-%d-%d.%s", firstImageID, e.TaskID, i, fileExt)
		}
		if sendPNGs {
			media = append(media, &models.InputMediaDocument{
				Media:           "attach://" + filename,
				MediaAttachment: bytes.NewReader(imgs[i]),
				ParseMode:       models.ParseModeHTML,
				Caption:         caption,
			})
		} else {
			media = append(media, &models.InputMediaPhoto{
				Media:           "attach://" + filename,
				MediaAttachment: bytes.NewReader(imgs[i]),
				ParseMode:       models.ParseModeHTML,
				Caption:         caption,
			})
		}
		caption = ""
	}

	err = e.bot.SendMediaGroup(ctx, e.Message, media)
	if err != nil {
		fmt.Println("  send images error:", err)

		if !retryAllowed {
			return fmt.Errorf("send images error: %w", err)
		}

		retryAfter := e.checkWaitError(err)
		if retryAfter > 0 {
			fmt.Println("  retrying after", retryAfter, "...")
			time.Sleep(retryAfter)
			return e.uploadImages(ctx, firstImageID, description, imgs, filename, false, sendPNGs)
		}
	}
	return nil
}

func (e *ReqQueueEntry) deleteReply(ctx context.Context) {
	if e.ReplyMessage == nil {
		return
	}

	_ = e.bot.DeleteMessage(ctx, e.ReplyMessage)
}

type ReqQueueCurrentEntry struct {
	entry     *ReqQueueEntry
	canceled  bool
	ctxCancel context.CancelFunc

	imgsChan    chan [][]byte
	errChan     chan error
	stoppedChan chan bool

	gotImageChan chan telegram.ImageFileData
}

type ReqQueue struct {
	bot            *telegram.SDBot
	mutex          sync.Mutex
	ctx            context.Context
	entries        []ReqQueueEntry
	processReqChan chan bool
	ProcessTimeout time.Duration

	currentEntry ReqQueueCurrentEntry
}

type ReqQueueReq struct {
	Type    ReqType
	Message *models.Message
	Params  reqparams.ReqParams
}

func (q *ReqQueue) CurrentEntryParams() reqparams.ReqParams {
	return q.currentEntry.entry.Params
}

func (q *ReqQueue) GotImage(ctx context.Context, updateMsg *models.Message, imageData *telegram.ImageFileData) {
	// Updating the message to reply to this document.
	q.currentEntry.entry.Message = updateMsg
	q.currentEntry.entry.ReplyMessage = nil
	// Notifying the request queue that we now got the image data.
	q.currentEntry.gotImageChan <- *imageData
}

func (q *ReqQueue) SendReplyToCurrentEntry(ctx context.Context, text string) {
	q.currentEntry.entry.sendReply(ctx, text)
}

func (q *ReqQueue) IsImageForMessage(msg *models.Message) bool {
	return q.currentEntry.gotImageChan != nil && msg.From.ID == q.currentEntry.entry.Message.From.ID
}

func (q *ReqQueue) IsCurrentEntryChat() bool {
	return q.currentEntry.entry.Message.Chat.ID >= 0
}

func (q *ReqQueue) Add(req ReqQueueReq) {
	q.mutex.Lock()

	newEntry := ReqQueueEntry{
		Type:   req.Type,
		Params: req.Params,
		TaskID: rand.Uint64(),

		bot:     q.bot,
		Message: req.Message,
	}

	if len(q.entries) > 0 {
		fmt.Println("  queueing request at position #", len(q.entries))
		newEntry.sendReply(q.ctx, q.getQueuePositionString(len(q.entries)))
	}

	q.entries = append(q.entries, newEntry)
	q.mutex.Unlock()

	select {
	case q.processReqChan <- true:
	default:
	}
}

func (q *ReqQueue) CancelCurrentEntry(ctx context.Context) (err error) {
	q.mutex.Lock()
	if len(q.entries) > 0 {
		q.currentEntry.canceled = true
		q.currentEntry.ctxCancel()
	} else {
		fmt.Println("  no active request to cancel")
		err = fmt.Errorf("no active request to cancel")
	}
	q.mutex.Unlock()
	return
}

func (q *ReqQueue) getQueuePositionString(pos int) string {
	return "👨‍👦‍👦 Request queued at position #" + fmt.Sprint(pos)
}

func (q *ReqQueue) queryProgress(ctx context.Context, sdApi *sdapi.SdAPIType, prevProgressPercent int) (progressPercent int, eta time.Duration, err error) {
	progressPercent = prevProgressPercent

	var newProgressPercent int
	newProgressPercent, eta, err = sdApi.GetProgress(ctx)
	if err == nil && newProgressPercent > prevProgressPercent {
		progressPercent = newProgressPercent
		if progressPercent > 100 {
			progressPercent = 100
		} else if progressPercent < 0 {
			progressPercent = 0
		}
		fmt.Print("    progress: ", progressPercent, "% eta: ", eta.Round(time.Second), "\n")
	}
	return
}

type ReqQueueEntryProcessFn func(context.Context, reqparams.ReqParams, []byte) (imgs [][]byte, err error)

func (q *ReqQueue) runProcessThread(processCtx context.Context, processFn ReqQueueEntryProcessFn, reqParams reqparams.ReqParams, imageData telegram.ImageFileData, retryAllowed bool,
	imgsChan chan [][]byte, errChan chan error, stoppedChan chan bool) {

	imgs, err := processFn(processCtx, reqParams, imageData.Data)
	if err == nil {
		imgsChan <- imgs
		stoppedChan <- true
		return
	}

	if errors.Is(err, syscall.ECONNREFUSED) { // Can't connect to Stable Diffusion?
		fmt.Println("  error: Stable Diffusion is not running and start is disabled, waiting...")
		time.Sleep(30 * time.Second)
		if retryAllowed {
			q.runProcessThread(processCtx, processFn, reqParams, imageData, false, imgsChan, errChan, stoppedChan)
			return
		}

		err = fmt.Errorf("error: Stable Diffusion is not running and start is disabled")
		fmt.Println("  error:", err)
	}

	errChan <- err
	stoppedChan <- true
}

func (q *ReqQueue) runProcess(
	processCtx context.Context,
	sdApi *sdapi.SdAPIType,
	processFn ReqQueueEntryProcessFn,
	reqParams reqparams.ReqParams,
	imageData telegram.ImageFileData,
	reqParamsText string,
) (imgs [][]byte, err error) {
	q.currentEntry.entry.sendReply(q.ctx, consts.ProcessStartStr+"\n"+reqParamsText)

	q.currentEntry.imgsChan = make(chan [][]byte)
	q.currentEntry.errChan = make(chan error, 1)
	q.currentEntry.stoppedChan = make(chan bool, 1)

	go q.runProcessThread(processCtx, processFn, reqParams, imageData, true, q.currentEntry.imgsChan, q.currentEntry.errChan, q.currentEntry.stoppedChan)
	fmt.Println("  render started")

	progressUpdateInterval := consts.GroupChatProgressUpdateInterval
	if q.currentEntry.entry.Message.Chat.ID >= 0 {
		progressUpdateInterval = consts.PrivateChatProgressUpdateInterval
	}
	progressPercentUpdateTicker := time.NewTicker(progressUpdateInterval)
	defer func() {
		progressPercentUpdateTicker.Stop()
		select {
		case <-progressPercentUpdateTicker.C:
		default:
		}
	}()
	progressCheckTicker := time.NewTicker(100 * time.Millisecond)
	defer func() {
		progressCheckTicker.Stop()
		select {
		case <-progressCheckTicker.C:
		default:
		}
	}()

	var progressPercent int
	var eta time.Duration
	for {
		select {
		case <-processCtx.Done():
			return nil, fmt.Errorf("timeout")
		case <-progressPercentUpdateTicker.C:
			q.currentEntry.entry.sendReply(q.ctx, consts.ProcessStr+" "+utils.GetProgressbar(progressPercent, consts.ProgressBarLength)+" ETA: "+fmt.Sprint(eta.Round(time.Second))+"\n"+reqParamsText)
		case <-progressCheckTicker.C:
			progressPercent, eta, _ = q.queryProgress(processCtx, sdApi, progressPercent)
		case err = <-q.currentEntry.errChan:
			return nil, err
		case imgs = <-q.currentEntry.imgsChan:
			return imgs, nil
		}
	}
}

func (q *ReqQueue) upscale(processCtx context.Context, sdApi *sdapi.SdAPIType, reqParams reqparams.ReqParamsUpscale, imageData telegram.ImageFileData) error {
	reqParamsText := reqParams.String()

	imgs, err := q.runProcess(processCtx, sdApi, sdApi.Upscale, reqParams, imageData, reqParamsText)
	if err != nil {
		return err
	}

	fn := utils.FilenameWithoutExt(imageData.Filename) + "-upscaled"
	if !reqParams.OutputPNG {
		err = q.currentEntry.entry.convertImagesFromPNGToJPG(imgs)
		if err != nil {
			return err
		}
		fn += ".jpg"
	} else {
		fn += ".png"
	}

	fmt.Println("  uploading...")
	q.currentEntry.entry.sendReply(q.ctx, consts.UploadingStr+"\n"+reqParamsText)

	err = q.currentEntry.entry.uploadImages(q.ctx, 0, "", imgs, fn, true, reqParams.OutputPNG)
	if err == nil {
		q.currentEntry.entry.deleteReply(q.ctx)
	}
	return err
}

func (q *ReqQueue) render(processCtx context.Context, sdApi *sdapi.SdAPIType, reqParams reqparams.ReqParamsRender) error {
	reqParamsText := reqParams.String()

	imgs, err := q.runProcess(processCtx, sdApi, sdApi.Render, reqParams, telegram.ImageFileData{}, reqParamsText)
	if err != nil {
		return err
	}

	// Now we have the output images.
	if reqParams.Upscale.Scale > 0 {
		reqParamsUpscale := reqparams.ReqParamsUpscale{
			OriginalPromptText: reqParams.OriginalPrompt(),
			Scale:              reqParams.Upscale.Scale,
			Upscaler:           reqParams.Upscale.Upscaler,
			OutputPNG:          reqParams.OutputPNG,
		}
		imgs, err = q.runProcess(processCtx, sdApi, sdApi.Upscale, reqParamsUpscale, telegram.ImageFileData{Data: imgs[0], Filename: ""}, reqParamsUpscale.String())
		if err != nil {
			return err
		}
	}

	if !reqParams.OutputPNG {
		err = q.currentEntry.entry.convertImagesFromPNGToJPG(imgs)
		if err != nil {
			return err
		}
	}

	fmt.Println("  uploading...")
	q.currentEntry.entry.sendReply(q.ctx, consts.UploadingStr+"\n"+reqParamsText)

	err = q.currentEntry.entry.uploadImages(q.ctx, reqParams.Seed, reqParams.OriginalPrompt()+"\n"+reqParamsText, imgs, "", true, reqParams.OutputPNG)
	if err == nil {
		q.currentEntry.entry.deleteReply(q.ctx)
	}
	return err
}

func (q *ReqQueue) processQueueEntry(processCtx context.Context, sdApi *sdapi.SdAPIType, imageData telegram.ImageFileData) error {
	fmt.Print("processing request from ", q.currentEntry.entry.Message.From.Username, "#",
		q.currentEntry.entry.Message.From.ID, ": ", q.currentEntry.entry.Params.OriginalPrompt(), "\n")

	switch q.currentEntry.entry.Type {
	case ReqTypeRender:
		return q.render(processCtx, sdApi, q.currentEntry.entry.Params.(reqparams.ReqParamsRender))
	case ReqTypeUpscale:
		return q.upscale(processCtx, sdApi, q.currentEntry.entry.Params.(reqparams.ReqParamsUpscale), imageData)
	case ReqTypeKuka:
		return q.kukafy(processCtx, sdApi, q.currentEntry.entry.Params.(reqparams.ReqParamsKuka), imageData, "MASK_CONST")
	default:
		return fmt.Errorf("unknown request type")
	}
}

// kukafy func like upscale with different processFn to execute custom model & promts for Kuka
func (q *ReqQueue) kukafy(processCtx context.Context, sdApi *sdapi.SdAPIType, reqParams reqparams.ReqParamsKuka, imageData telegram.ImageFileData, maskPrompt string) error {
	reqParamsText := reqParams.String()

	// Create a mask from the image based on the prompt
	_, err := createMaskFromPrompt(imageData.Filename, maskPrompt)
	if err != nil {
		return fmt.Errorf("error creating mask: %w", err)
	}

	// Use Img2Img with inpainting
	imgs, err := q.runProcess(processCtx, sdApi, func(ctx context.Context, p reqparams.ReqParams, imgData []byte) ([][]byte, error) {
		return sdApi.Img2Img(ctx, p, imgData)
	}, reqParams, imageData, reqParamsText)
	if err != nil {
		return err
	}

	fn := utils.FilenameWithoutExt(imageData.Filename) + "-kukafied"
	if !reqParams.OutputPNG {
		err = q.currentEntry.entry.convertImagesFromPNGToJPG(imgs)
		if err != nil {
			return err
		}
		fn += ".jpg"
	} else {
		fn += ".png"
	}

	fmt.Println("  uploading...")
	q.currentEntry.entry.sendReply(q.ctx, consts.UploadingStr+"\n"+reqParamsText)

	err = q.currentEntry.entry.uploadImages(q.ctx, 0, "", imgs, fn, true, reqParams.OutputPNG)
	if err == nil {
		q.currentEntry.entry.deleteReply(q.ctx)
	}
	return err
}

// FeatPart represents which part of the Feat to detect
type FeatPart string

const (
	FeatTop    FeatPart = "top"
	FeatBottom FeatPart = "bottom"
)

// createMaskFromPrompt generates a binary mask focusing on Feat detection
// filename: path to the input image
// prompt: text description of the Feat to mask (e.g., "red Feat top", "blue Feat bottom")
func createMaskFromPrompt(filename, prompt string) ([]byte, error) {
	// Read the input image
	img := gocv.IMRead(filename, gocv.IMReadColor)
	if img.Empty() {
		return nil, errors.New("error reading image file")
	}
	defer img.Close()

	// Create a blank mask of the same size as input image
	finalMask := gocv.NewMatWithSize(img.Rows(), img.Cols(), gocv.MatTypeCV8UC1)
	defer finalMask.Close()

	// Detect person to limit the search area
	personMask, err := detectPerson(img)
	if err != nil {
		return nil, err
	}
	defer personMask.Close()

	// Determine which part of the Feat to detect
	var part FeatPart
	if containsKeyword(prompt, "top", "upper") {
		part = FeatTop
	} else if containsKeyword(prompt, "bottom", "lower") {
		part = FeatBottom
	} else {
		// Default to detecting both parts
		part = FeatTop
	}

	// Create Feat mask based on the prompt and part
	FeatMask, err := detectFeat(img, personMask, prompt, part)
	if err != nil {
		return nil, err
	}
	defer FeatMask.Close()

	// Combine masks
	gocv.BitwiseAnd(personMask, FeatMask, &finalMask)

	// Convert Mat to byte slice
	bytes := finalMask.ToBytes()

	return bytes, nil
}

func detectPerson(img gocv.Mat) (gocv.Mat, error) {
	mask := gocv.NewMatWithSize(img.Rows(), img.Cols(), gocv.MatTypeCV8UC1)

	// Use body detection optimized for swimwear
	hog := gocv.NewHOGDescriptor()
	defer hog.Close()
	hog.SetSVMDetector(gocv.HOGDefaultPeopleDetector())

	// Detect with HOG
	regions := hog.DetectMultiScale(img)
	for _, r := range regions {
		gocv.Rectangle(&mask, r, color.RGBA{R: 255, G: 255, B: 255, A: 0}, -1) // Updated line
	}

	return mask, nil
}

func detectFeat(img, personMask gocv.Mat, prompt string, part FeatPart) (gocv.Mat, error) {
	mask := gocv.NewMatWithSize(img.Rows(), img.Cols(), gocv.MatTypeCV8UC1)

	// Convert to different color spaces for better segmentation
	hsvImg := gocv.NewMat()
	labImg := gocv.NewMat()
	defer hsvImg.Close()
	defer labImg.Close()

	gocv.CvtColor(img, &hsvImg, gocv.ColorBGRToHSV)
	gocv.CvtColor(img, &labImg, gocv.ColorBGRToLab)

	// Get color-based mask
	colorMask := getFeatColorMask(hsvImg, prompt)
	defer colorMask.Close()

	// Get region-based mask
	regionMask := getFeatRegionMask(img, part)
	defer regionMask.Close()

	// Combine color and region masks
	gocv.BitwiseAnd(colorMask, regionMask, &mask)

	// Detect patterns if specified in prompt
	if containsKeyword(prompt, "pattern", "print", "stripe", "dot") {
		patternMask := detectPattern(img)
		defer patternMask.Close()
		gocv.BitwiseAnd(mask, patternMask, &mask)
	}

	// Post-processing specific to swimwear
	refineMask(img, &mask, part)

	return mask, nil
}

func getFeatColorMask(hsvImg gocv.Mat, prompt string) gocv.Mat {
	mask := gocv.NewMatWithSize(hsvImg.Rows(), hsvImg.Cols(), gocv.MatTypeCV8UC1)

	// Define expanded color ranges for swimwear
	colorRanges := map[string]struct {
		lower gocv.Scalar
		upper gocv.Scalar
	}{
		"red":    {gocv.NewScalar(0.0, 50.0, 50.0, 0.0), gocv.NewScalar(10.0, 255.0, 255.0, 0.0)},
		"blue":   {gocv.NewScalar(100.0, 50.0, 50.0, 0.0), gocv.NewScalar(130.0, 255.0, 255.0, 0.0)},
		"green":  {gocv.NewScalar(45.0, 50.0, 50.0, 0.0), gocv.NewScalar(75.0, 255.0, 255.0, 0.0)},
		"pink":   {gocv.NewScalar(150.0, 50.0, 50.0, 0.0), gocv.NewScalar(170.0, 255.0, 255.0, 0.0)},
		"purple": {gocv.NewScalar(130.0, 50.0, 50.0, 0.0), gocv.NewScalar(150.0, 255.0, 255.0, 0.0)},
		"yellow": {gocv.NewScalar(20.0, 50.0, 50.0, 0.0), gocv.NewScalar(30.0, 255.0, 255.0, 0.0)},
		"orange": {gocv.NewScalar(10.0, 50.0, 50.0, 0.0), gocv.NewScalar(20.0, 255.0, 255.0, 0.0)},
		"black":  {gocv.NewScalar(0.0, 0.0, 0.0, 0.0), gocv.NewScalar(180.0, 255.0, 50.0, 0.0)},
		"white":  {gocv.NewScalar(0.0, 0.0, 200.0, 0.0), gocv.NewScalar(180.0, 30.0, 255.0, 0.0)},
	}

	for color, range_ := range colorRanges {
		if containsKeyword(prompt, color) {
			colorMask := gocv.NewMat()
			defer colorMask.Close()
			lowerBound := gocv.NewMatWithSize(hsvImg.Rows(), hsvImg.Cols(), gocv.MatTypeCV8UC1)
			defer lowerBound.Close()
			upperBound := gocv.NewMatWithSize(hsvImg.Rows(), hsvImg.Cols(), gocv.MatTypeCV8UC1)
			defer upperBound.Close()
			lowerBound.SetTo(range_.lower)
			upperBound.SetTo(range_.upper)
			gocv.InRange(hsvImg, lowerBound, upperBound, &colorMask)
			gocv.BitwiseOr(mask, colorMask, &mask)
		}
	}

	return mask
}

func getFeatRegionMask(img gocv.Mat, part FeatPart) gocv.Mat {
	mask := gocv.NewMatWithSize(img.Rows(), img.Cols(), gocv.MatTypeCV8UC1)

	switch part {
	case FeatTop:
		// Focus on upper body region (approximately 25-40% of height)
		// Adjusted for typical Feat top placement
		roi := image.Rect(
			img.Cols()/6,   // x start
			img.Rows()/4,   // y start
			img.Cols()*5/6, // x end
			img.Rows()*2/5, // y end
		)
		gocv.Rectangle(&mask, roi, color.RGBA{R: 255, G: 255, B: 255, A: 0}, -1)

	case FeatBottom:
		// Focus on lower body region (approximately 50-65% of height)
		// Adjusted for typical Feat bottom placement
		roi := image.Rect(
			img.Cols()/6,     // x start
			img.Rows()/2,     // y start
			img.Cols()*5/6,   // x end
			img.Rows()*13/20, // y end
		)
		gocv.Rectangle(&mask, roi, color.RGBA{R: 255, G: 255, B: 255, A: 0}, -1)
	}

	return mask
}

func detectPattern(img gocv.Mat) gocv.Mat {
	mask := gocv.NewMatWithSize(img.Rows(), img.Cols(), gocv.MatTypeCV8UC1)

	// Convert to grayscale
	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)

	// Edge detection for patterns
	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(gray, &edges, 50, 150)

	// Detect repeating patterns using frequency analysis
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
	defer kernel.Close()
	gocv.MorphologyEx(edges, &mask, gocv.MorphClose, kernel)

	return mask
}

func refineMask(img gocv.Mat, mask *gocv.Mat, part FeatPart) {
	// Apply different refinement based on Feat part
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
	defer kernel.Close()

	switch part {
	case FeatTop:
		// More aggressive cleaning for top part
		gocv.MorphologyEx(*mask, mask, gocv.MorphOpen, kernel)
		gocv.MorphologyEx(*mask, mask, gocv.MorphClose, kernel)

	case FeatBottom:
		// Less aggressive cleaning for bottom part
		gocv.MorphologyEx(*mask, mask, gocv.MorphClose, kernel)
	}

	// Edge refinement
	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(img, &edges, 100, 200)
	gocv.BitwiseAnd(*mask, edges, mask)
}

func containsKeyword(prompt string, keywords ...string) bool {
	promptLower := strings.ToLower(prompt)
	for _, keyword := range keywords {
		if strings.Contains(promptLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func (q *ReqQueue) processor(sdApi *sdapi.SdAPIType) {
	for {
		q.mutex.Lock()
		if (len(q.entries)) == 0 {
			q.mutex.Unlock()
			<-q.processReqChan
			continue
		}

		// Updating queue positions for all waiting entries.
		for i := 1; i < len(q.entries); i++ {
			q.bot.SendReplyToMessage(q.ctx, q.entries[i].Message, q.getQueuePositionString(i))
		}

		q.currentEntry = ReqQueueCurrentEntry{
			entry: &q.entries[0],
		}
		var processCtx context.Context
		processCtx, q.currentEntry.ctxCancel = context.WithTimeout(q.ctx, q.ProcessTimeout)
		q.mutex.Unlock()

		var err error
		var imageData telegram.ImageFileData
		imageNeededFirst := false
		switch q.currentEntry.entry.Type {
		case ReqTypeUpscale:
			imageNeededFirst = true
		case ReqTypeKuka:
			imageNeededFirst = true
		}
		if imageNeededFirst {
			fmt.Println("  waiting for image file...")
			q.currentEntry.entry.sendReply(q.ctx, consts.ImageReqStr)
			q.currentEntry.gotImageChan = make(chan telegram.ImageFileData)
			select {
			case imageData = <-q.currentEntry.gotImageChan:
			case <-processCtx.Done():
				q.currentEntry.canceled = true
			case <-time.NewTimer(3 * time.Minute).C:
				fmt.Println("  waiting for image file timeout")
				err = fmt.Errorf("waiting for image data timeout")
			}
			close(q.currentEntry.gotImageChan)
			q.currentEntry.gotImageChan = nil

			if err == nil && len(imageData.Data) == 0 {
				err = fmt.Errorf("got no image data")
			}
		}

		if err == nil {
			err = q.processQueueEntry(processCtx, sdApi, imageData)
		}

		q.mutex.Lock()
		if q.currentEntry.canceled {
			fmt.Print("  canceled\n")
			err = sdApi.Interrupt(q.ctx)
			if err != nil {
				fmt.Println("  can't interrupt:", err)
			}
			q.currentEntry.entry.sendReply(q.ctx, consts.CanceledStr)
		} else if err != nil {
			fmt.Println("  error:", err)
			q.currentEntry.entry.sendReply(q.ctx, consts.ErrorStr+": "+err.Error())
		}

		q.currentEntry.ctxCancel()

		if q.currentEntry.stoppedChan != nil {
			<-q.currentEntry.stoppedChan
			close(q.currentEntry.imgsChan)
			close(q.currentEntry.errChan)
			close(q.currentEntry.stoppedChan)
			q.currentEntry.stoppedChan = nil
		}

		q.entries = q.entries[1:]
		if len(q.entries) == 0 {
			fmt.Print("finished queue processing\n")
		}
		q.mutex.Unlock()
	}
}

func (q *ReqQueue) Init(ctx context.Context, sdApi *sdapi.SdAPIType, bot *telegram.SDBot) {
	q.ctx = ctx
	q.processReqChan = make(chan bool)
	q.bot = bot
	go q.processor(sdApi)
}
