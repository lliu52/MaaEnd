package dailyrewardtelegram

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/rs/zerolog/log"
)

const (
	actionName  = "DailyRewardTelegramScreenshot"
	component   = "DailyRewardTelegramScreenshot"
	sendTimeout = 20 * time.Second
	jpegQuality = 90
)

const screenshotCaption = "✅ 日常奖励领取完成\n请检查左侧 100 活跃度奖励是否已领取。"

// Action captures the settled daily-task page and sends it to Telegram.
// Notification failures are best-effort and must not turn a completed game task into a failure.
type Action struct {
	sender photoSender
}

var _ maa.CustomActionRunner = (*Action)(nil)

func (a *Action) Run(ctx *maa.Context, _ *maa.CustomActionArg) bool {
	if a.sender == nil {
		log.Warn().Str("component", component).Msg("Telegram is not configured; daily reward screenshot was not sent")
		return true
	}
	if ctx == nil || ctx.GetTasker() == nil || ctx.GetTasker().GetController() == nil {
		log.Error().Str("component", component).Msg("Cannot capture daily reward screenshot: controller is unavailable")
		return true
	}

	controller := ctx.GetTasker().GetController()
	controller.PostScreencap().Wait()
	screenshot, err := controller.CacheImage()
	if err != nil {
		log.Error().Err(err).Str("component", component).Msg("Failed to capture daily reward screenshot")
		return true
	}
	photo, err := encodeScreenshot(screenshot)
	if err != nil {
		log.Error().Err(err).Str("component", component).Msg("Failed to encode daily reward screenshot")
		return true
	}

	filename := fmt.Sprintf("daily-reward-%s.jpg", time.Now().Format("20060102-150405"))
	sendCtx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	err = a.sender.SendPhoto(sendCtx, photo, filename, screenshotCaption)
	cancel()
	if err != nil {
		log.Warn().Err(err).Str("component", component).Msg("Failed to send daily reward screenshot to Telegram")
		return true
	}

	log.Info().
		Str("component", component).
		Int("photo_bytes", len(photo)).
		Msg("Daily reward screenshot sent to Telegram")
	return true
}

func encodeScreenshot(screenshot image.Image) ([]byte, error) {
	if screenshot == nil {
		return nil, fmt.Errorf("screenshot is nil")
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, screenshot, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encode screenshot as JPEG: %w", err)
	}
	return encoded.Bytes(), nil
}
