package reqparams

import (
	"fmt"
)

type ReqParamsImg2Img struct {
	HR                 ReqParamsRenderHR
	OriginalPromptText string
	Prompt             string
	NegativePrompt     string
	Seed               uint32
	Width              int
	Height             int
	BatchSize          int
	Steps              int
	NumOutputs         int
	OutputPNG          bool
	CFGScale           float64
	SamplerName        string
	ModelName          string
}

func (r ReqParamsImg2Img) String() string {
	var numOutputs string
	if r.NumOutputs > 1 {
		numOutputs = fmt.Sprintf("x%d", r.NumOutputs)
	}

	var outFormatText string
	if r.OutputPNG {
		outFormatText = "/PNG"
	}

	res := fmt.Sprintf("🌱<code>%d</code> 👟%d 🕹%.1f 🖼%dx%d%s%s 🔭%s 🧩%s",
		r.Seed,
		r.Steps,
		r.CFGScale,
		r.Width,
		r.Height,
		numOutputs,
		outFormatText,
		r.SamplerName,
		r.ModelName,
	)

	if r.NegativePrompt != "" {
		negText := r.NegativePrompt
		if len(negText) > 10 {
			negText = negText[:10] + "..."
		}
		res = "📍" + negText + " " + res
	}

	return res
}

func (r ReqParamsImg2Img) OriginalPrompt() string {
	return r.OriginalPromptText
}

type ReqParamsUpscale struct {
	OriginalPromptText string
	Scale              float32
	Upscaler           string
	OutputPNG          bool
}

func (r ReqParamsUpscale) String() string {
	res := "🔎 " + r.Upscaler + "x" + fmt.Sprint(r.Scale)
	if r.OutputPNG {
		res += "/PNG"
	}
	return res
}

func (r ReqParamsUpscale) OriginalPrompt() string {
	return r.OriginalPromptText
}

type ReqParamsRenderHR struct {
	DenoisingStrength float32
	Scale             float32
	Upscaler          string
	SecondPassSteps   int
}

type ReqParamsRender struct {
	OriginalPromptText string
	Prompt             string
	NegativePrompt     string
	Seed               uint32
	Width              int
	Height             int
	BatchSize          int
	Steps              int
	NumOutputs         int
	OutputPNG          bool
	CFGScale           float64
	SamplerName        string
	ModelName          string

	Upscale ReqParamsUpscale

	HR ReqParamsRenderHR

	EnableHR          bool
	DenoisingStrength float32
	HRScale           float32
	HRUpscaler        string
	HRSecondPassSteps int
}

func (r ReqParamsRender) String() string {
	var numOutputs string
	if r.NumOutputs > 1 {
		numOutputs = fmt.Sprintf("x%d", r.NumOutputs)
	}

	var outFormatText string
	if r.OutputPNG {
		outFormatText = "/PNG"
	}

	res := fmt.Sprintf("🌱<code>%d</code> 👟%d 🕹%.1f 🖼%dx%d%s%s 🔭%s 🧩%s",
		r.Seed,
		r.Steps,
		r.CFGScale,
		r.Width,
		r.Height,
		numOutputs,
		outFormatText,
		r.SamplerName,
		r.ModelName,
	)

	if r.HR.Scale > 0 {
		res += " 🔎 " + r.HR.Upscaler + "x" + fmt.Sprint(r.HR.Scale, "/", r.HR.DenoisingStrength)
	} else if r.Upscale.Scale > 0 {
		res += " " + r.Upscale.String()
	}

	if r.NegativePrompt != "" {
		negText := r.NegativePrompt
		if len(negText) > 10 {
			negText = negText[:10] + "..."
		}
		res = "📍" + negText + " " + res
	}
	return res
}

func (r ReqParamsRender) OriginalPrompt() string {
	return r.OriginalPromptText
}

type ReqParams interface {
	String() string
	OriginalPrompt() string
}
