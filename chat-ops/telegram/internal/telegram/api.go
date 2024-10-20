package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type SDBot struct {
	bot        *bot.Bot
	getFileUrl func(fileInfo *models.File) string
}

func NewBot(botToken string, defailtHandlerFunc bot.HandlerFunc) (*SDBot, error) {
	botInternal, err := bot.New(botToken, bot.WithDefaultHandler(defailtHandlerFunc))
	if err != nil {
		return nil, fmt.Errorf("cannot create telegram bot with token: %w", err)
	}
	return &SDBot{bot: botInternal, getFileUrl: func(fileInfo *models.File) string {
		return fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileInfo.FilePath)
	}}, nil
}

func (b *SDBot) RegisterPrefixHandler(pattern string, handlerFunc bot.HandlerFunc) string {
	return b.bot.RegisterHandler(bot.HandlerTypeMessageText, pattern, bot.MatchTypePrefix, handlerFunc)
}

func (b *SDBot) Start(ctx context.Context) {
	b.bot.Start(ctx)
}

func (b *SDBot) SendReplyToMessage(ctx context.Context, replyToMsg *models.Message, text string) (msg *models.Message) {
	var err error
	msg, err = b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ReplyToMessageID: replyToMsg.ID,
		ChatID:           replyToMsg.Chat.ID,
		ParseMode:        models.ParseModeHTML,
		Text:             text,
	})
	if err != nil {
		fmt.Println("  reply send error:", err)
	}
	return
}

func (b *SDBot) EditMessage(ctx context.Context, editableMsg *models.Message, newText string) error {
	_, err := b.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		MessageID: editableMsg.ID,
		ChatID:    editableMsg.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      newText,
	})
	return err
}

func (b *SDBot) DeleteMessage(ctx context.Context, deletingMessage *models.Message) error {
	_, err := b.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
		MessageID: deletingMessage.ID,
		ChatID:    deletingMessage.Chat.ID,
	})
	return err
}

func (b *SDBot) SendTextToAdmins(ctx context.Context, adminUserIds []int64, s string) {
	for _, chatID := range adminUserIds {
		_, _ = b.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   s,
		})
	}
}

func (b *SDBot) SendMediaGroup(ctx context.Context, replyToMsg *models.Message, media []models.InputMedia) error {
	_, err := b.bot.SendMediaGroup(ctx, &bot.SendMediaGroupParams{
		ChatID:           replyToMsg.Chat.ID,
		ReplyToMessageID: replyToMsg.ID,
		Media:            media,
	})
	return err

}
func (b *SDBot) GetFile(ctx context.Context, fileId string, getWriterFunc func(fileSize int64) io.Writer) (d []byte, err error) {
	fmt.Println("  downloading...")

	fileInfo, err := b.bot.GetFile(ctx, &bot.GetFileParams{
		FileID: fileId,
	})
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	resp, err := http.Get(b.getFileUrl(fileInfo))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	writer := getWriterFunc(fileInfo.FileSize)

	d, err = io.ReadAll(io.TeeReader(resp.Body, writer))
	if err != nil {
		return nil, err
	}

	fmt.Println("  downloading done")
	return d, nil
}

type ImageFileData struct {
	Data     []byte
	Filename string
}
