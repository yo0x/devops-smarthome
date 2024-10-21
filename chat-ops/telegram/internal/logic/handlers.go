package logic

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os/exec"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/config"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/consts"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/logic/userservice"
	reqqueue "github.com/kanootoko/stable-diffusion-telegram-bot/internal/req_queue"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/reqparams"
	sdapi "github.com/kanootoko/stable-diffusion-telegram-bot/internal/sd_api"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/telegram"
)

func NewCmdHandler(
	sdApi *sdapi.SdAPIType,
	reqQueue *reqqueue.ReqQueue,
	generationDefaults config.GenerationDefaults,
	userService userservice.UserService,
) *CmdHandler {
	c := CmdHandler{
		sdApi:    sdApi,
		reqQueue: reqQueue,
		defaults: generationDefaults,
		us:       userService,
	}
	return &c
}

func (c *CmdHandler) AddHandlers(
	bot *telegram.SDBot,
) {
	c.bot = bot
	bot.RegisterPrefixHandler("/start", c.adaptHandler(c.Start))
	bot.RegisterPrefixHandler("/sd", c.adaptHandler(c.txt2img))
	bot.RegisterPrefixHandler("/txt2img", c.adaptHandler(c.txt2img))
	bot.RegisterPrefixHandler("/upscale", c.adaptHandler(c.upscale))
	bot.RegisterPrefixHandler("/cancel", c.adaptHandler(c.cancel))
	bot.RegisterPrefixHandler("/smi", c.adaptHandler(c.smi))
	bot.RegisterPrefixHandler("/help", c.adaptHandler(c.help))
	bot.RegisterPrefixHandler("/kuka", c.adaptHandler(c.img2img))

	bot.RegisterPrefixHandler("/models", c.adaptHandler(c.listModels))
	bot.RegisterPrefixHandler("/samplers", c.adaptHandler(c.listSamplers))
	bot.RegisterPrefixHandler("/embeddings", c.adaptHandler(c.listEmbeddings))
	bot.RegisterPrefixHandler("/loras", c.adaptHandler(c.listLoRAs))
	bot.RegisterPrefixHandler("/upscalers", c.adaptHandler(c.listUpscalers))
	bot.RegisterPrefixHandler("/vaes", c.adaptHandler(c.listVAEs))
}

func (c *CmdHandler) GetDefaultHandler() bot.HandlerFunc {
	return c.adaptHandler(c.defaultHandler)
}

func removeBotName(s string) string {
	if s == "" {
		return s
	}
	if s[0] == '/' {
		spaceIndex := strings.Index(s, " ")
		if spaceIndex == -1 {
			return ""
		} else {
			return s[spaceIndex:]
		}
	}
	return s
}

func (c *CmdHandler) adaptHandler(innerHandler func(context.Context, *models.Message)) bot.HandlerFunc {
	return func(ctx context.Context, _ *bot.Bot, update *models.Update) {
		if update.Message == nil { // edited message is ignored
			return
		}
		fmt.Print("msg from ", update.Message.From.Username, "#", update.Message.From.ID, ": ", update.Message.Text, "\n")
		if update.Message.Chat.ID < 0 { // From group chat?
			fmt.Print("  msg from group #", update.Message.Chat.ID)
		}
		fmt.Println()

		if update.Message.ReplyToMessage != nil &&
			update.Message.Text != "" &&
			update.Message.Text[0] != '/' {
			fmt.Println("  skipping message as a reply to bot without a command")
			return
		}

		if !c.us.IsUsageAllowed(update.Message.From.ID, update.Message.Chat.ID) {
			fmt.Println("  user not allowed, ignoring")
			if update.Message.Text != "" && update.Message.Text[0] == '/' || update.Message.From.ID == update.Message.Chat.ID {
				c.bot.SendReplyToMessage(ctx, update.Message, consts.UsageNotAllowedStr)
			}
			return
		}

		innerHandler(ctx, update.Message)
	}
}

type CmdHandler struct {
	sdApi    *sdapi.SdAPIType
	bot      *telegram.SDBot
	reqQueue *reqqueue.ReqQueue
	defaults config.GenerationDefaults
	//defaultEnv config.DefaultsFromEnv
	us userservice.UserService
}

func (c *CmdHandler) img2img(ctx context.Context, msg *models.Message) {
	log.Println("DEBUG: img2img command received")

	reqParams := reqparams.ReqParamsKuka{
		OriginalPromptText: c.defaults.KukaPrompt,
		Prompt:             c.defaults.KukaPrompt,
		NegativePrompt:     c.defaults.KukaNegativePrompt,
		Seed:               rand.Uint32(),
		Width:              c.defaults.Width,
		Height:             c.defaults.Height,
		Steps:              c.defaults.KukaSteps,
		NumOutputs:         1,
		CFGScale:           c.defaults.KukaCFGScale,
		SamplerName:        c.defaults.Sampler,
		ModelName:          c.defaults.KukaModel,
	}

	log.Printf("DEBUG: Kuka params: %+v", reqParams)

	req := reqqueue.ReqQueueReq{
		Type:    reqqueue.ReqTypeKuka,
		Message: msg,
		Params:  reqParams,
	}

	c.bot.SendReplyToMessage(ctx, msg, consts.ImageReqStr)
	c.reqQueue.Add(req)
}

func (c *CmdHandler) txt2img(ctx context.Context, msg *models.Message) {
	text := strings.TrimSpace(removeBotName(msg.Text))
	reqParams := reqparams.ReqParamsRender{
		OriginalPromptText: text,
		Seed:               rand.Uint32(),
		Width:              c.defaults.Width,
		Height:             c.defaults.Height,
		Steps:              c.defaults.Steps,
		NumOutputs:         c.defaults.Cnt,
		CFGScale:           c.defaults.CFGScale,
		SamplerName:        c.defaults.Sampler,
		ModelName:          c.defaults.Model,
		Upscale: reqparams.ReqParamsUpscale{
			Upscaler: "LDSR",
		},
		HR: reqparams.ReqParamsRenderHR{
			DenoisingStrength: 0.4,
			Upscaler:          "R-ESRGAN 4x+",
			SecondPassSteps:   15,
		},
	}
	log.Printf("DEBUG: Parsed params: %+v", reqParams)
	var paramsLine *string
	lines := strings.Split(text, "\n")
	if len(lines) > 1 {
		reqParams.Prompt = lines[0]
		reqParams.NegativePrompt = strings.Join(lines[1:], " ")
		paramsLine = &reqParams.NegativePrompt
		log.Printf("DEBUG: Negative prompt: %s", reqParams.NegativePrompt)
	} else {
		reqParams.Prompt = text
		paramsLine = &reqParams.Prompt
		log.Printf("DEBUG: Prompt: %s", reqParams.Prompt)
	}
	firstCmdCharAt, err := ReqParamsParse(ctx, c.sdApi, c.defaults, *paramsLine, &reqParams)
	if err != nil {
		log.Printf("ERROR: Failed to parse render params: %v", err)
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": can't parse render params: "+err.Error())
		return
	}
	if firstCmdCharAt >= 0 { // Commands found? Removing them from the line.
		if firstCmdCharAt == 0 {
			log.Println("WARN: Empty request error")
			c.bot.SendReplyToMessage(ctx, msg, consts.EmptyRequestErrorStr)
			return
		}
		*paramsLine = (*paramsLine)[:firstCmdCharAt]
		if len(lines) > 1 {
			firstCmdCharAt += len(lines[0]) + 1
			log.Printf("DEBUG: First command char at: %d", firstCmdCharAt)
		}
		reqParams.OriginalPromptText = fmt.Sprintf("%s\nParameters: %s", reqParams.OriginalPromptText[:firstCmdCharAt], reqParams.OriginalPromptText[firstCmdCharAt:])
		log.Printf("DEBUG: Original prompt text: %s", reqParams.OriginalPromptText)
	}

	reqParams.Prompt = strings.TrimSpace(reqParams.Prompt)
	reqParams.NegativePrompt = strings.TrimSpace(reqParams.NegativePrompt)
	log.Printf("DEBUG: Final params: %+v", reqParams)
	if reqParams.Prompt == "" {
		log.Println("WARN: Missing prompt")
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": missing prompt")
		return
	}

	if reqParams.HR.Scale > 0 || reqParams.Upscale.Scale > 0 {
		log.Println("DEBUG: Setting num outputs to 1")
		reqParams.NumOutputs = 1
	}

	req := reqqueue.ReqQueueReq{
		Type:    reqqueue.ReqTypeRender,
		Message: msg,
		Params:  reqParams,
	}
	log.Println("DEBUG: adding req: ", req)
	c.reqQueue.Add(req)

}

func (c *CmdHandler) upscale(ctx context.Context, msg *models.Message) {
	reqParams := reqparams.ReqParamsUpscale{
		OriginalPromptText: msg.Text,
		Scale:              2,
		Upscaler:           "LDSR",
	}

	firstCmdCharAt, err := ReqParamsParse(ctx, c.sdApi, c.defaults, msg.Text, &reqParams)
	if err != nil {
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": can't parse render params: "+err.Error())
		return
	}
	if firstCmdCharAt >= 0 {
		reqParams.OriginalPromptText = fmt.Sprintf("%s\nParameters: %s", reqParams.OriginalPromptText[:firstCmdCharAt], reqParams.OriginalPromptText[firstCmdCharAt:])
	}

	req := reqqueue.ReqQueueReq{
		Type:    reqqueue.ReqTypeUpscale,
		Message: msg,
		Params:  reqParams,
	}
	c.reqQueue.Add(req)
}

func (c *CmdHandler) cancel(ctx context.Context, msg *models.Message) {
	if err := c.reqQueue.CancelCurrentEntry(ctx); err != nil {
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": "+err.Error())
	}
}

func (c *CmdHandler) listModels(ctx context.Context, msg *models.Message) {
	models, err := c.sdApi.GetModels(ctx)
	if err != nil {
		fmt.Println("  error getting models:", err)
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": error getting models: "+err.Error())
		return
	}
	for i := range models {
		if models[i] == c.defaults.Model {
			models[i] = "- <b>" + models[i] + "</b> (default)"
		} else {
			models[i] = "- <code>" + models[i] + "</code>"
		}
	}
	res := strings.Join(models, "\n")
	var text string
	if res != "" {
		text = "🧩 Available models:\n" + res
	} else {
		text = "No available models."
	}
	c.bot.SendReplyToMessage(ctx, msg, text)
}

func (c *CmdHandler) listSamplers(ctx context.Context, msg *models.Message) {
	samplers, err := c.sdApi.GetSamplers(ctx)
	if err != nil {
		fmt.Println("  error getting samplers:", err)
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": error getting samplers: "+err.Error())
		return
	}
	for i := range samplers {
		if samplers[i] == c.defaults.Sampler {
			samplers[i] = "- <b>" + samplers[i] + "</b> (default)"
		} else {
			samplers[i] = "- <code>" + samplers[i] + "</code>"
		}
	}
	res := strings.Join(samplers, "\n")
	var text string
	if res != "" {
		text = "🔭 Available samplers:\n" + res
	} else {
		text = "No available samplers."
	}
	c.bot.SendReplyToMessage(ctx, msg, text)
}

func (c *CmdHandler) listEmbeddings(ctx context.Context, msg *models.Message) {
	embs, err := c.sdApi.GetEmbeddings(ctx)
	if err != nil {
		fmt.Println("  error getting embeddings:", err)
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": error getting embeddings: "+err.Error())
		return
	}
	for i := range embs {
		embs[i] = "- <code>" + embs[i] + "</code>"
	}
	res := strings.Join(embs, "\n")
	var text string
	if res != "" {
		text = "Available embeddings: " + res
	} else {
		text = "No available embeddings."
	}
	c.bot.SendReplyToMessage(ctx, msg, text)
}

func (c *CmdHandler) listLoRAs(ctx context.Context, msg *models.Message) {
	loras, err := c.sdApi.GetLoRAs(ctx)
	if err != nil {
		fmt.Println("  error getting loras:", err)
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": error getting loras: "+err.Error())
		return
	}
	for i := range loras {
		loras[i] = "- <code>" + loras[i] + "</code>"
	}
	res := strings.Join(loras, "\n")
	var text string
	if res != "" {
		text = "Available LoRAs: " + res
	} else {
		text = "No available LoRAs."
	}
	c.bot.SendReplyToMessage(ctx, msg, text)
}

func (c *CmdHandler) listUpscalers(ctx context.Context, msg *models.Message) {
	ups, err := c.sdApi.GetUpscalers(ctx)
	if err != nil {
		fmt.Println("  error getting upscalers:", err)
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": error getting upscalers: "+err.Error())
		return
	}
	for i := range ups {
		ups[i] = "- <code>" + ups[i] + "</code>"
	}
	res := strings.Join(ups, "\n")
	var text string
	if res != "" {
		text = "🔎 Available upscalers: " + res
	} else {
		text = "🔎 No available upscalers."
	}
	c.bot.SendReplyToMessage(ctx, msg, text)
}

func (c *CmdHandler) listVAEs(ctx context.Context, msg *models.Message) {
	vaes, err := c.sdApi.GetVAEs(ctx)
	if err != nil {
		fmt.Println("  error getting vaes:", err)
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": error getting vaes: "+err.Error())
		return
	}
	for i := range vaes {
		vaes[i] = "- <code>" + vaes[i] + "</code>"
	}
	res := strings.Join(vaes, "\n")
	var text string
	if res != "" {
		text = "Available VAEs: " + res
	} else {
		text = "No available VAEs."
	}
	c.bot.SendReplyToMessage(ctx, msg, text)
}

func (c *CmdHandler) smi(ctx context.Context, msg *models.Message) {
	cmd := exec.Command("nvidia-smi")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("  error running nvidia-smi:", err)
		c.bot.SendReplyToMessage(ctx, msg, consts.ErrorStr+": error running nvidia-smi: "+err.Error())
		return
	}
	c.bot.SendReplyToMessage(ctx, msg, "<pre>"+string(out)+"</pre>")
}

func (c *CmdHandler) Start(ctx context.Context, msg *models.Message) {
	if msg.Chat.ID >= 0 {
		c.bot.SendReplyToMessage(ctx, msg, consts.StartStr)
	}
}

func (c *CmdHandler) help(ctx context.Context, msg *models.Message) {
	c.bot.SendReplyToMessage(
		ctx,
		msg,
		consts.HelpCommandStr,
	)
}

func (c *CmdHandler) defaultHandler(ctx context.Context, msg *models.Message) {
	if msg.Document != nil {
		c.handleImage(ctx, msg, msg.Document.FileID, msg.Document.FileName)
		return
	} else if msg.Photo != nil && len(msg.Photo) > 0 {
		c.handleImage(ctx, msg, msg.Photo[len(msg.Photo)-1].FileID, "image.jpg")
		return
	}
	if msg.Chat.ID >= 0 {
		c.txt2img(ctx, msg)
	}
}

func (c *CmdHandler) handleImage(ctx context.Context, msg *models.Message, fileID, filename string) {
	// Are we expecting image data from this user?
	if !c.reqQueue.IsImageForMessage(msg) {
		return
	}

	counter := &WriteCounter{
		Ctx:                   ctx,
		TotalBytes:            0,
		ProgressPrintInterval: consts.GroupChatProgressUpdateInterval,
		reqQueue:              c.reqQueue,
	}

	if c.reqQueue.IsCurrentEntryChat() {
		counter.ProgressPrintInterval = consts.PrivateChatProgressUpdateInterval
	}

	d, err := c.bot.GetFile(ctx, fileID, func(fileSize int64) io.Writer {
		counter.TotalBytes = fileSize
		return counter
	})

	if err != nil {
		c.reqQueue.SendReplyToCurrentEntry(ctx, consts.ErrorStr+": can't get file: "+err.Error())
		return
	}

	c.reqQueue.SendReplyToCurrentEntry(ctx, consts.DoneStr+" downloading\n"+c.reqQueue.CurrentEntryParams().String())
	c.reqQueue.GotImage(ctx, msg, &telegram.ImageFileData{
		Data:     d,
		Filename: filename,
	})
}
