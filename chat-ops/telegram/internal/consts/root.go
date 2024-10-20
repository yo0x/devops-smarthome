package consts

import "time"

const ImageReqStr = "🩻 Please send the image file to process."
const ProcessStartStr = "🛎 Starting render..."
const ProcessStr = "🔨 Working"
const ProgressBarLength = 16
const DownloadingStr = "⬇ Downloading..."
const UploadingStr = "☁ Uploading..."
const DoneStr = "✅ Done"
const ErrorStr = "❌ Error"
const CanceledStr = "⭕ Canceled"
const StartStr = "🤖 Welcome! This is a Telegram Bot " +
	"for rendering images with Stable Diffusion.\n\nMore info:" +
	" https://github.com/kanootoko/stable-diffusion-telegram-bot"
const BotStartedToAdminsStr = "🤖 Bot started, version "
const UsageNotAllowedStr = "You need to contact bot hoster to enable the functionality"
const EmptyRequestErrorStr = "Request is empty, generation skipped"

const HelpCommandStr = "🤖 Stable Diffusion Telegram Bot\n\n" +
	"Available commands:\n\n" +

	"/sd [prompt] - render prompt (negative prompt can be put" +
	" on the next line)\n" +
	"/upscale - upscale image\n" +
	"/cancel - cancel ongoing request\n" +
	"/models - list available models\n" +
	"/samplers - list available samplers\n" +
	"/embeddings - list available embeddings\n" +
	"/loras - list available LoRAs\n" +
	"/upscalers - list available upscalers\n" +
	"/vaes - list available VAEs\n" +
	"/smi - get the output of nvidia-smi\n" +
	"/help - show this help\n\n" +
	"/kuka - img2img with prompt with teaks and model kuka\n" +

	"Available render parameters at the end of the prompt:\n\n" +

	"-seed/s - set seed\n" +
	"-width/w - set output image width\n" +
	"-height/h - set output image height\n" +
	"-steps/t - set the number of steps\n" +
	"-cnt/o - set count of output images\n" +
	"-batch/b - set batch size of output images\n" +
	"-png - upload PNGs instead of JPEGs\n" +
	"-cfg/c - set CFG scale\n" +
	"-sampler/r - set sampler, get valid values with /samplers\n" +
	"-model/m - set model, get valid values with /models\n" +
	"-upscale/u - upscale output image with ratio\n" +
	"-upscaler - set upscaler method, get valid values with /upscalers\n" +
	"-hr - enable highres mode and set upscale ratio\n" +
	"-hr-denoisestrength/hrd - set highres mode denoise strength\n" +
	"-hr-upscaler/hru - set highres mode upscaler, get valid values with /upscalers\n" +
	"-hr-steps/hrt - set the number of highres mode second pass steps\n\n" +

	"Available upscale parameters:\n\n" +

	"-upscale/u - upscale output image with ratio\n" +
	"-upscaler - set upscaler method, get valid values with /upscalers\n" +
	"-png - upload PNGs instead of JPEGs\n\n" +

	"For more information see https://github.com/kanootoko/stable-diffusion-telegram-bot"

const GroupChatProgressUpdateInterval = 5 * time.Second
const PrivateChatProgressUpdateInterval = 3 * time.Second
