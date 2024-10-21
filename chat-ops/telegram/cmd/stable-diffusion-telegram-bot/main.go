package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/kanootoko/stable-diffusion-telegram-bot/internal"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/config"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/consts"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/logic"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/logic/userservice"
	reqqueue "github.com/kanootoko/stable-diffusion-telegram-bot/internal/req_queue"
	sdapi "github.com/kanootoko/stable-diffusion-telegram-bot/internal/sd_api"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/telegram"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/utils"
)

func main() {
	fmt.Println("stable-diffusion-telegram-bot starting...")
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if _, isEnvFileSet := os.LookupEnv("ENVFILE"); isEnvFileSet {
		utils.ReadEnvFile(os.Getenv("ENVFILE"))
	} else {
		utils.ReadEnvFile(".env")
	}

	var params config.AppParams

	if err := params.Init(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	log.Println("Using params", params)
	var cancel context.CancelFunc
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	sdApi := sdapi.SdAPIType{SdHost: params.StableDiffusionApiHost}
	log.Println("sdApi.SdHost", sdApi.SdHost)
	reqQueue := reqqueue.ReqQueue{ProcessTimeout: params.ProcessTimeout}
	cmdHandler := logic.NewCmdHandler(
		&sdApi,
		&reqQueue,
		params.Defaults,
		userservice.NewUserServiceStatic(params.AllowedUserIDs, params.AllowedGroupIDs, params.AdminUserIDs),
	)

	telegramBot, err := telegram.NewBot(params.BotToken, cmdHandler.GetDefaultHandler())

	if nil != err {
		panic(fmt.Sprint("can't init telegram bot: ", err.Error()))
	}
	log.Println("telegramBot", telegramBot)
	cmdHandler.AddHandlers(telegramBot)

	reqQueue.Init(ctx, &sdApi, telegramBot)

	verStr, _ := sdapi.VersionCheckGetStr(ctx, params.StableDiffusionApiHost)
	telegramBot.SendTextToAdmins(ctx, params.AdminUserIDs, consts.BotStartedToAdminsStr+internal.Version+", "+verStr)
	log.Println("Bot started")
	go func() {
		for {
			time.Sleep(24 * time.Hour)
			if s, updateNeededOrError := sdapi.VersionCheckGetStr(ctx, params.StableDiffusionApiHost); updateNeededOrError {
				telegramBot.SendTextToAdmins(ctx, params.AdminUserIDs, s)
			}
		}
	}()
	telegramBot.Start(ctx)
}
