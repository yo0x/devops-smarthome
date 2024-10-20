package sdapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/reqparams"
)

type SdAPIType struct {
	SdHost string
}

func (a *SdAPIType) req(ctx context.Context, path, service string, postData []byte) (string, error) {
	path, err := url.JoinPath(a.SdHost, "/sdapi/v1", path)
	if err != nil {
		return "", err
	}

	path += service

	var request *http.Request
	if postData != nil {
		request, err = http.NewRequestWithContext(ctx, "POST", path, bytes.NewBuffer(postData))
		if err != nil {
			return "", err
		}
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	} else {
		request, err = http.NewRequestWithContext(ctx, "GET", path, nil)
		if err != nil {
			return "", err
		}
	}

	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Printf("Error on body: %s", postData)
		if bodyBytes, err := io.ReadAll(resp.Body); err == nil {
			fmt.Printf("Response: %s\n", string(bodyBytes))
		} else {
			fmt.Printf("")
		}
		return "", fmt.Errorf("api status code: %d (%s to %s)", resp.StatusCode, request.Method, path)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

type RenderReq struct {
	EnableHR          bool                   `json:"enable_hr"`
	DenoisingStrength float32                `json:"denoising_strength"`
	HRScale           float32                `json:"hr_scale"`
	HRUpscaler        string                 `json:"hr_upscaler"`
	HRSecondPassSteps int                    `json:"hr_second_pass_steps"`
	HRSamplerName     string                 `json:"hr_sampler_name"`
	HRPrompt          string                 `json:"hr_prompt"`
	HRNegativePrompt  string                 `json:"hr_negative_prompt"`
	Prompt            string                 `json:"prompt"`
	Seed              uint32                 `json:"seed"`
	SamplerName       string                 `json:"sampler_name"`
	BatchSize         int                    `json:"batch_size"`
	NIter             int                    `json:"n_iter"`
	Steps             int                    `json:"steps"`
	CFGScale          float64                `json:"cfg_scale"`
	Width             int                    `json:"width"`
	Height            int                    `json:"height"`
	NegativePrompt    string                 `json:"negative_prompt"`
	OverrideSettings  map[string]interface{} `json:"override_settings"`
	SendImages        bool                   `json:"send_images"`
}

func (a *SdAPIType) Render(ctx context.Context, p reqparams.ReqParams, _ []byte) (imgs [][]byte, err error) {
	params := p.(reqparams.ReqParamsRender)

	n_iter := int(math.Ceil(float64(params.NumOutputs) / float64(params.BatchSize)))

	postData, err := json.Marshal(RenderReq{
		EnableHR:          params.HR.Scale > 0,
		DenoisingStrength: params.HR.DenoisingStrength,
		HRScale:           params.HR.Scale,
		HRUpscaler:        params.HR.Upscaler,
		HRSecondPassSteps: params.HR.SecondPassSteps,
		HRSamplerName:     params.SamplerName,
		HRPrompt:          params.Prompt,
		HRNegativePrompt:  params.NegativePrompt,
		Prompt:            params.Prompt,
		Seed:              params.Seed,
		SamplerName:       params.SamplerName,
		BatchSize:         params.BatchSize,
		NIter:             n_iter,
		Steps:             params.Steps,
		CFGScale:          params.CFGScale,
		Width:             params.Width,
		Height:            params.Height,
		NegativePrompt:    params.NegativePrompt,
		OverrideSettings: map[string]interface{}{
			"sd_model_checkpoint": params.ModelName,
		},
		SendImages: true,
	})
	if err != nil {
		return nil, err
	}

	res, err := a.req(ctx, "/txt2img", "", postData)
	if err != nil {
		return nil, err
	}

	var renderResp struct {
		Images []string `json:"images"`
	}
	err = json.Unmarshal([]byte(res), &renderResp)
	if err != nil {
		return nil, err
	}
	if len(renderResp.Images) == 0 {
		return nil, fmt.Errorf("unknown error")
	}

	for _, img := range renderResp.Images {
		var unbased []byte
		if unbased, err = base64.StdEncoding.DecodeString(img); err != nil {
			return nil, fmt.Errorf("image base64 decode error")
		}
		imgs = append(imgs, unbased)
	}

	return imgs, nil
}

type UpscaleReq struct {
	ResizeMode                     int     `json:"resize_mode,omitempty"`
	ShowExtrasResults              bool    `json:"show_extras_results,omitempty"`
	GFPGANVisibility               float32 `json:"gfpgan_visibility,omitempty"`
	CodeFormerVisibility           float32 `json:"codeformer_visibility,omitempty"`
	CodeFormerWeight               float32 `json:"codeformer_weight,omitempty"`
	UpscalingResize                float32 `json:"upscaling_resize,omitempty"`
	UpscalingResizeWidth           int     `json:"upscaling_resize_w,omitempty"`
	UpscalingResizeHeight          int     `json:"upscaling_resize_h,omitempty"`
	UpscalingResizeWidthHeightCrop bool    `json:"upscaling_crop,omitempty"`
	Upscaler1                      string  `json:"upscaler_1,omitempty"`
	Upscaler2                      string  `json:"upscaler_2,omitempty"`
	Upscaler2Visibility            float32 `json:"extras_upscaler_2_visibility,omitempty"`
	UpscaleFirst                   bool    `json:"upscale_first,omitempty"`
	Image                          string  `json:"image"`
}

func (a *SdAPIType) Upscale(ctx context.Context, p reqparams.ReqParams, imageData []byte) (imgs [][]byte, err error) {
	params := p.(reqparams.ReqParamsUpscale)

	postData, err := json.Marshal(UpscaleReq{
		UpscalingResize: params.Scale,
		Upscaler1:       params.Upscaler,
		Image:           base64.StdEncoding.EncodeToString(imageData),
	})
	if err != nil {
		return nil, err
	}

	res, err := a.req(ctx, "/extra-single-image", "", postData)
	if err != nil {
		return nil, err
	}

	var renderResp struct {
		Image string `json:"image"`
	}
	err = json.Unmarshal([]byte(res), &renderResp)
	if err != nil {
		return nil, err
	}
	if len(renderResp.Image) == 0 {
		return nil, fmt.Errorf("unknown error")
	}

	var unbased []byte
	if unbased, err = base64.StdEncoding.DecodeString(renderResp.Image); err != nil {
		return nil, fmt.Errorf("image base64 decode error")
	}

	return [][]byte{unbased}, nil
}

func (a *SdAPIType) Interrupt(ctx context.Context) error {
	_, err := a.req(ctx, "/interrupt", "", []byte{})
	if err != nil {
		return err
	}
	return nil
}

func (a *SdAPIType) GetProgress(ctx context.Context) (progressPercent int, eta time.Duration, err error) {
	res, err := a.req(ctx, "/progress", "?skip_current_image=false", nil)
	if err != nil {
		return 0, 0, err
	}

	var progressRes struct {
		Progress float32 `json:"progress"`
		ETA      float32 `json:"eta_relative"`
		Detail   string  `json:"detail"`
	}
	err = json.Unmarshal([]byte(res), &progressRes)
	if err != nil {
		return 0, 0, err
	}

	if progressRes.Detail != "" {
		return 0, 0, fmt.Errorf(progressRes.Detail)
	}

	return int(progressRes.Progress * 100), time.Duration(progressRes.ETA * float32(time.Second)), nil
}

func (a *SdAPIType) GetModels(ctx context.Context) (models []string, err error) {
	res, err := a.req(ctx, "/sd-models", "", nil)
	if err != nil {
		return nil, err
	}

	var modelsRes []struct {
		Name string `json:"model_name"`
	}
	err = json.Unmarshal([]byte(res), &modelsRes)
	if err != nil {
		return nil, err
	}

	for _, m := range modelsRes {
		models = append(models, m.Name)
	}
	return
}

func (a *SdAPIType) GetSamplers(ctx context.Context) (samplers []string, err error) {
	res, err := a.req(ctx, "/samplers", "", nil)
	if err != nil {
		return nil, err
	}

	var samplersRes []struct {
		Name string `json:"name"`
	}
	err = json.Unmarshal([]byte(res), &samplersRes)
	if err != nil {
		return nil, err
	}

	for _, sampler := range samplersRes {
		samplers = append(samplers, sampler.Name)
	}
	return
}

func (a *SdAPIType) GetEmbeddings(ctx context.Context) (embs []string, err error) {
	res, err := a.req(ctx, "/embeddings", "", nil)
	if err != nil {
		return nil, err
	}

	var embList struct {
		Loaded map[string]struct{} `json:"loaded"`
	}
	err = json.Unmarshal([]byte(res), &embList)
	if err != nil {
		return nil, err
	}

	for i := range embList.Loaded {
		embs = append(embs, i)
	}
	return
}

func (a *SdAPIType) GetLoRAs(ctx context.Context) (loras []string, err error) {
	res, err := a.req(ctx, "/loras", "", nil)
	if err != nil {
		return nil, err
	}

	var lorasRes []struct {
		Name string `json:"name"`
	}
	err = json.Unmarshal([]byte(res), &lorasRes)
	if err != nil {
		return nil, err
	}

	for _, lora := range lorasRes {
		loras = append(loras, lora.Name)
	}
	return
}

func (a *SdAPIType) GetUpscalers(ctx context.Context) (upscalers []string, err error) {
	res, err := a.req(ctx, "/upscalers", "", nil)
	if err != nil {
		return nil, err
	}

	var upscalersRes []struct {
		Name string `json:"name"`
	}
	err = json.Unmarshal([]byte(res), &upscalersRes)
	if err != nil {
		return nil, err
	}

	for _, u := range upscalersRes {
		upscalers = append(upscalers, u.Name)
	}
	return
}

func (a *SdAPIType) GetVAEs(ctx context.Context) (vaes []string, err error) {
	res, err := a.req(ctx, "/sd-vae", "", nil)
	if err != nil {
		return nil, err
	}

	var vaesRes []struct {
		Name string `json:"model_name"`
	}
	err = json.Unmarshal([]byte(res), &vaesRes)
	if err != nil {
		return nil, err
	}

	for _, u := range vaesRes {
		vaes = append(vaes, u.Name)
	}
	return
}
