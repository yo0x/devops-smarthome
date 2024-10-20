# Stable Diffusion Telegram Bot

This is a Telegram Bot frontend for rendering images with
[Stable Diffusion AUTOMATIC1111 API](https://github.com/AUTOMATIC1111/stable-diffusion-webui/).

<p align="center"><img src="resources/demo.gif?raw=true"/></p>

The bot displays the progress and further information during processing by
responding to the message with the prompt. Requests are queued, only one gets
processed at a time.

The bot uses the
[Telegram Bot API](https://github.com/go-telegram-bot-api/telegram-bot-api).
Rendered images are not saved on disk.

## Compiling

You'll need Go installed on your computer. Install a recent package of [Go](https://go.dev).
Then run:

```shell
go install github.com/kanootoko/stable-diffusion-telegram-bot/cmd/stable-diffusion-telegram-bot@latest
```

This will typically install `stable-diffusion-telegram-bot` into `~/go/bin`.

Or just enter `go build` in the cloned Git source repo directory.

## Prerequisites

Create a Telegram bot using [BotFather](https://t.me/BotFather) and get the
bot's `token`.

## Running

You can get the available command line arguments with `-h`.
Mandatory arguments are:

- `-bot-token`: set this to your Telegram bot's `token`
- `-sd-api`: set the address of running Stable Diffusion AUTOMATIC1111 API

Set your Telegram user ID as an admin with the `-admin-user-ids` argument.
Admins will get a message when the bot starts.

Other user/group IDs can be set with the `-allowed-user-ids` and
`-allowed-group-ids` arguments. IDs should be separated by commas.

You can get Telegram user IDs by writing a message to the bot and checking
the app's log, as it logs all incoming messages.

All command line arguments can be set through OS environment variables.
Note that using a command line argument overwrites a setting by the environment
variable. Available OS environment variables are listed in [.env example file](.env.example).

## Bot operation

Supported commands listed in [commands.txt file](commands.txt). You can also set 
commands suggestions from Telegram using BotFather by feeding it with
[commands file](./docs/resources/commands.txt) content.

When sending message in private chat, any message which is not a command will be treated as
a generation request.

### Setting render parameters

You can use the following `-attr val` assignments at the end of the prompt:

- `-seed/s` - set seed
- `-width/w` - set output image width
- `-height/h` - set output image height
- `-steps/t` - set the number of steps
- `-cnt/o` - set count of output images
- `-batch/b` - set batch size of output images
- `-png` - upload PNGs instead of JPEGs
- `-cfg/c` - set CFG scale
- `-sampler/r` - set sampler, get valid values with `/samplers`
- `-model/m` - set model, get valid values with `/models`
- `-upscale/u` - upscale output image with ratio
- `-upscaler` - set upscaler method, get valid values with `/upscalers`
- `-hr` - enable highres mode and set upscale ratio
- `-hr-denoisestrength/hrd` - set highres mode denoise strength
- `-hr-upscaler/hru` - set highres mode upscaler, get valid values with `/upscalers`
- `-hr-steps/hrt` - set the number of highres mode second pass steps

Example prompt with attributes: `laughing santa with beer -s 1 -o 1`

Enter negative prompts in the second line of your message (use Shift+Enter). Example:
```
laughing santa with beer
tree -s 1 -o 1
```

If you need to use spaces in sampler and upscaler names, then enclose them
in double quotes.

The default resolution is 512x512. If the currently used model's name contains "xl" as
case-insensitive substring then the bot increases the resolution to the other one
(default is 1024x1024).
