package reqqueue

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"math/rand"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/consts"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/reqparams"
	sdapi "github.com/kanootoko/stable-diffusion-telegram-bot/internal/sd_api"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/telegram"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/utils"
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
		return q.kukafy(processCtx, sdApi, q.currentEntry.entry.Params.(reqparams.ReqParamsKuka), imageData)
	default:
		return fmt.Errorf("unknown request type")
	}
}

// kukafy func like upscale with different processFn to execute custom model & promts for Kuka
func (q *ReqQueue) kukafy(processCtx context.Context, sdApi *sdapi.SdAPIType, reqParams reqparams.ReqParamsKuka, imageData telegram.ImageFileData) error {
	reqParamsText := reqParams.String()

	imgs, err := q.runProcess(processCtx, sdApi, sdApi.Img2Img, reqParams, imageData, reqParamsText)
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
