package logic

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/shlex"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/config"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/reqparams"
	sdapi "github.com/kanootoko/stable-diffusion-telegram-bot/internal/sd_api"
	"golang.org/x/exp/slices"
)

// Returns -1 as firstCmdCharAt if no params have been found in the given string.
func ReqParamsParse(ctx context.Context, sdApi *sdapi.SdAPIType, defaults config.GenerationDefaults, s string, reqParams reqparams.ReqParams) (firstCmdCharAt int, err error) {
	lexer := shlex.NewLexer(strings.NewReader(s))

	var reqParamsRender *reqparams.ReqParamsRender
	var reqParamsUpscale *reqparams.ReqParamsUpscale
	switch v := reqParams.(type) {
	case *reqparams.ReqParamsRender:
		reqParamsRender = v
	case *reqparams.ReqParamsUpscale:
		reqParamsUpscale = v
	default:
		return 0, fmt.Errorf("invalid reqParams type")
	}

	gotWidth := false
	gotHeight := false
	gotSteps := false
	gotNumOutputs := false
	gotBatchSize := false

	firstCmdCharAt = -1
	for {
		token, lexErr := lexer.Next()
		if lexErr != nil { // No more tokens?
			break
		}

		if token[0] != '-' {
			if firstCmdCharAt > -1 {
				return 0, fmt.Errorf("params need to be after the prompt")
			}
			continue // Ignore tokens not starting with -
		}

		attr := strings.ToLower(token[1:])
		validAttr := false

		switch attr {
		case "seed", "s":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			val = strings.TrimPrefix(val, "🌱")
			valInt, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid seed")
			}
			reqParamsRender.Seed = uint32(valInt)
			validAttr = true
		case "width", "w":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid width")
			}
			reqParamsRender.Width = valInt
			validAttr = true
			gotWidth = true
		case "height", "h":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid height")
			}
			reqParamsRender.Height = valInt
			validAttr = true
			gotHeight = true
		case "steps", "t":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid steps")
			}
			reqParamsRender.Steps = valInt
			validAttr = true
			gotSteps = true
		case "batch", "b":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid batch size")
			}
			reqParamsRender.BatchSize = valInt
			validAttr = true
			gotBatchSize = true
		case "cnt", "o":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid output count")
			}
			reqParamsRender.NumOutputs = valInt
			validAttr = true
			gotNumOutputs = true
		case "png", "p":
			if reqParamsRender != nil {
				reqParamsRender.OutputPNG = true
			} else if reqParamsUpscale != nil {
				reqParamsUpscale.OutputPNG = true
			}
			validAttr = true
		case "cfg", "c":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return 0, fmt.Errorf("  invalid CFG scale")
			}
			reqParamsRender.CFGScale = valFloat
			validAttr = true
		case "sampler", "r":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			samplers, err := sdApi.GetSamplers(ctx)
			if err != nil {
				return 0, fmt.Errorf("error getting samplers: %w", err)
			}
			if !slices.Contains(samplers, val) {
				return 0, fmt.Errorf("invalid sampler")
			}
			reqParamsRender.SamplerName = val
			validAttr = true
		case "model", "m":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			models, err := sdApi.GetModels(ctx)
			if err != nil {
				return 0, fmt.Errorf("error getting models: %w", err)
			}
			if !slices.Contains(models, val) {
				return 0, fmt.Errorf(" invalid model")
			}
			reqParamsRender.ModelName = val
			validAttr = true
		case "upscale", "u":
			if reqParamsRender == nil && reqParamsUpscale == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid hr scale")
			}
			if reqParamsRender != nil {
				reqParamsRender.Upscale.Scale = float32(valFloat)
			} else if reqParamsUpscale != nil {
				reqParamsUpscale.Scale = float32(valFloat)
			}
			validAttr = true
		case "upscaler":
			if reqParamsRender == nil && reqParamsUpscale == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			upscalers, err := sdApi.GetUpscalers(ctx)
			if err != nil {
				return 0, fmt.Errorf("error getting upscalers: %w", err)
			}
			if !slices.Contains(upscalers, val) {
				return 0, fmt.Errorf("invalid upscaler")
			}
			if reqParamsRender != nil {
				reqParamsRender.Upscale.Upscaler = val
			} else if reqParamsUpscale != nil {
				reqParamsUpscale.Upscaler = val
			}
			validAttr = true
		case "hr":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid hr scale")
			}
			reqParamsRender.HR.Scale = float32(valFloat)
			validAttr = true
		case "hr-denoisestrength", "hrd":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid hr denoise strength")
			}
			reqParamsRender.HR.DenoisingStrength = float32(valFloat)
			validAttr = true
		case "hr-upscaler", "hru":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			upscalers, err := sdApi.GetUpscalers(ctx)
			if err != nil {
				return 0, fmt.Errorf("error getting upscalers: %w", err)
			}
			if !slices.Contains(upscalers, val) {
				return 0, fmt.Errorf("invalid upscaler")
			}
			reqParamsRender.HR.Upscaler = val
			validAttr = true
		case "hr-steps", "hrt":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid hr second pass steps")
			}
			reqParamsRender.HR.SecondPassSteps = valInt
			validAttr = true
		}

		if validAttr && firstCmdCharAt == -1 {
			firstCmdCharAt = strings.Index(s, token)
		}
	}

	if reqParamsRender != nil {
		if !gotNumOutputs {
			reqParamsRender.NumOutputs = defaults.Cnt
		}
		if !gotBatchSize {
			reqParamsRender.BatchSize = defaults.Batch
		}
		if strings.Contains(strings.ToLower(reqParamsRender.ModelName), "xl") {
			if !gotWidth {
				reqParamsRender.Width = defaults.WidthSDXL
			}
			if !gotHeight {
				reqParamsRender.Height = defaults.HeightSDXL
			}
			if !gotSteps {
				reqParamsRender.Steps = defaults.Steps
			}
		} else {
			if !gotWidth {
				reqParamsRender.Width = defaults.Width
			}
			if !gotHeight {
				reqParamsRender.Height = defaults.Height
			}
			if !gotSteps {
				reqParamsRender.Steps = defaults.StepsSDXL
			}
		}

		// Don't allow upscaler while HR is enabled.
		if reqParamsRender.HR.Scale > 0 {
			reqParamsRender.Upscale.Scale = 0
		}
	}

	return
}
