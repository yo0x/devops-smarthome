package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/consts"
	reqqueue "github.com/kanootoko/stable-diffusion-telegram-bot/internal/req_queue"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/utils"
)

// WriteCounter counts the number of bytes written to it. It implements to the io.Writer interface
// and we can pass this into io.TeeReader() which will report progress on each write cycle.
type WriteCounter struct {
	Ctx                   context.Context
	GotBytes              int64
	TotalBytes            int64
	ProgressPrintInterval time.Duration
	LastProgressPrintAt   time.Time
	reqQueue              *reqqueue.ReqQueue
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.GotBytes += int64(n)

	if time.Since(wc.LastProgressPrintAt) > wc.ProgressPrintInterval {
		progressPercent := int(float64(wc.GotBytes) / float64(wc.TotalBytes) * 100)
		fmt.Print("    progress: ", progressPercent, "%\n")
		wc.reqQueue.SendReplyToCurrentEntry(wc.Ctx, consts.DownloadingStr+" "+utils.GetProgressbar(progressPercent, consts.ProgressBarLength))
		wc.LastProgressPrintAt = time.Now()
	}
	return n, nil
}
