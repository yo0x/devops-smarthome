package config

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type GenerationDefaults struct {
	Model      string
	Sampler    string
	Cnt        int
	Batch      int
	Steps      int
	Width      int
	Height     int
	WidthSDXL  int
	HeightSDXL int
	StepsSDXL  int
	CFGScale   float64
}

func (d GenerationDefaults) String() string {
	return fmt.Sprintf(
		"{model: %s, sampler: %s, cnt: %d, batch: %d, steps: %d, width: %d, height: %d, widthXL: %d, heightXL: %d, stepsXL: %d, cfg: %.2f}",
		d.Model,
		d.Sampler,
		d.Cnt,
		d.Batch,
		d.Steps,
		d.Width,
		d.Height,
		d.WidthSDXL,
		d.HeightSDXL,
		d.StepsSDXL,
		d.CFGScale,
	)
}

type AppParams struct {
	StableDiffusionApiHost string

	BotToken        string
	AllowedUserIDs  []int64
	AdminUserIDs    []int64
	AllowedGroupIDs []int64
	ProcessTimeout  time.Duration

	Defaults GenerationDefaults
}

func (p AppParams) String() string {
	return fmt.Sprintf(
		"{sdAPI: %s, token: ...%s, admins: %v, allowedUsers: %v, allowedGroups: %v, processTimeout: %v, defaults: %v}",
		p.StableDiffusionApiHost,
		p.BotToken[max(len(p.BotToken)-4, 0):],
		p.AdminUserIDs,
		p.AllowedUserIDs,
		p.AllowedGroupIDs,
		p.ProcessTimeout,
		p.Defaults,
	)
}

func (p *AppParams) Init() error {

	defaults := getDefaultsFromEnv()

	flag.StringVar(&p.BotToken, "bot-token", "", "telegram bot token [required]")
	flag.StringVar(&p.StableDiffusionApiHost, "sd-api", defaults.StableDiffusionApiHost, "address of running Stable Diffusion AUTOMATIC1111 API")
	var allowedUserIDs string
	flag.StringVar(&allowedUserIDs, "allowed-user-ids", defaults.AllowedUserIDs, "allowed telegram user ids")
	var adminUserIDs string
	flag.StringVar(&adminUserIDs, "admin-user-ids", defaults.AdminUserIDs, "admin telegram user ids")
	var allowedGroupIDs string
	flag.StringVar(&allowedGroupIDs, "allowed-group-ids", defaults.AllowedGroupIDs, "allowed telegram group ids")
	flag.DurationVar(&p.ProcessTimeout, "process-timeout", defaults.ProcessTimeout, "maximum time before generation auto-cancel")
	flag.StringVar(&p.Defaults.Model, "default-model", defaults.Model, "default model name")
	flag.StringVar(&p.Defaults.Sampler, "default-sampler", defaults.Sampler, "default sampler name")
	flag.IntVar(&p.Defaults.Cnt, "default-cnt", defaults.Cnt, "default images count")
	flag.IntVar(&p.Defaults.Batch, "default-batch", defaults.Batch, "default images batch size")
	flag.IntVar(&p.Defaults.Steps, "default-steps", defaults.Steps, "default generation steps")
	flag.IntVar(&p.Defaults.Width, "default-width", defaults.Width, "default image width")
	flag.IntVar(&p.Defaults.Height, "default-height", defaults.Height, "default image height")
	flag.IntVar(&p.Defaults.WidthSDXL, "default-width-sdxl", defaults.WidthSDXL, "default image width for SDXL models")
	flag.IntVar(&p.Defaults.HeightSDXL, "default-height-sdxl", defaults.HeightSDXL, "default image height for SDXL models")
	flag.IntVar(&p.Defaults.StepsSDXL, "default-cnt-sdxl", defaults.StepsSDXL, "default generation steps count for SDXL models")
	flag.Float64Var(&p.Defaults.CFGScale, "default-cfg-scale", defaults.CFGScale, "default CFG scale")
	flag.StringVar(&p.Defaults.Model, "default-model", defaults.Model, "default model name")
	flag.Parse()

	if p.BotToken == "" {
		p.BotToken = os.Getenv("BOT_TOKEN")
	}
	if p.BotToken == "" {
		return fmt.Errorf("bot token not set")
	}

	sa := strings.Split(allowedUserIDs, ",")
	for _, idStr := range sa {
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("allowed user ids contains invalid user ID: " + idStr)
		}
		p.AllowedUserIDs = append(p.AllowedUserIDs, id)
	}

	sa = strings.Split(adminUserIDs, ",")
	for _, idStr := range sa {
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("admin ids contains invalid user ID: " + idStr)
		}
		p.AdminUserIDs = append(p.AdminUserIDs, id)
		if !slices.Contains(p.AllowedUserIDs, id) {
			p.AllowedUserIDs = append(p.AllowedUserIDs, id)
		}
	}

	sa = strings.Split(allowedGroupIDs, ",")
	for _, idStr := range sa {
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("allowed group ids contains invalid group ID: " + idStr)
		}
		p.AllowedGroupIDs = append(p.AllowedGroupIDs, id)
	}
	return nil
}

type defaultsFromEnv struct {
	StableDiffusionApiHost string
	Model                  string
	Sampler                string
	Cnt                    int
	Batch                  int
	Steps                  int
	Width                  int
	Height                 int
	WidthSDXL              int
	HeightSDXL             int
	StepsSDXL              int
	CFGScale               float64
	AllowedUserIDs         string
	AdminUserIDs           string
	AllowedGroupIDs        string
	ProcessTimeout         time.Duration
	KukaPrompt             string
	KukaNegativePrompt     string
	KukaModel              string
}

func getDefaultsFromEnv() (defaults defaultsFromEnv) {
	if value, isSet := os.LookupEnv("STABLE_DIFFUSION_API"); isSet {
		defaults.StableDiffusionApiHost = value
	} else {
		defaults.StableDiffusionApiHost = "http://localhost:7860"
	}

	if value, isSet := os.LookupEnv("DEFAULT_MODEL"); isSet {
		defaults.Model = value
	}

	if value, isSet := os.LookupEnv("DEFAULT_SAMPLER"); isSet {
		defaults.Sampler = value
	}
	if value, isSet := os.LookupEnv("DEFAULT_KUKA_PROMPT"); isSet {
		defaults.KukaPrompt = value
	}
	if value, isSet := os.LookupEnv("DEFAULT_KUKA_NEGATIVE_PROMPT"); isSet {
		defaults.KukaNegativePrompt = value
	}
	if value, isSet := os.LookupEnv("DEFAULT_KUKA_MODEL"); isSet {
		defaults.KukaModel = value
	}
	if value, isSet := os.LookupEnv("DEFAULT_WIDTH"); isSet {
		if intValue, err := strconv.Atoi(value); err == nil {
			defaults.Width = intValue
		} else {
			defaults.Width = 512
		}
	} else {
		defaults.Width = 512
	}

	if value, isSet := os.LookupEnv("DEFAULT_HEIGHT"); isSet {
		if intValue, err := strconv.Atoi(value); err == nil {
			defaults.Height = intValue
		} else {
			defaults.Height = 512
		}
	} else {
		defaults.Height = 512
	}

	if value, isSet := os.LookupEnv("DEFAULT_STEPS"); isSet {
		if intValue, err := strconv.Atoi(value); err == nil {
			defaults.Steps = intValue
		} else {
			defaults.Steps = 30
		}
	} else {
		defaults.Steps = 30
	}

	if value, isSet := os.LookupEnv("DEFAULT_CNT"); isSet {
		if intValue, err := strconv.Atoi(value); err == nil {
			defaults.Cnt = intValue
		} else {
			defaults.Cnt = 2
		}
	} else {
		defaults.Cnt = 2
	}

	if value, isSet := os.LookupEnv("DEFAULT_BATCH"); isSet {
		if intValue, err := strconv.Atoi(value); err == nil {
			defaults.Batch = intValue
		} else {
			defaults.Batch = 1
		}
	} else {
		defaults.Batch = 1
	}

	if value, isSet := os.LookupEnv("DEFAULT_WIDTH_SDXL"); isSet {
		if intValue, err := strconv.Atoi(value); err == nil {
			defaults.WidthSDXL = intValue
		} else {
			defaults.WidthSDXL = 512
		}
	} else {
		defaults.WidthSDXL = 512
	}

	if value, isSet := os.LookupEnv("DEFAULT_HEIGHT_SDXL"); isSet {
		if intValue, err := strconv.Atoi(value); err == nil {
			defaults.HeightSDXL = intValue
		} else {
			defaults.HeightSDXL = 512
		}
	} else {
		defaults.HeightSDXL = 512
	}

	if value, isSet := os.LookupEnv("DEFAULT_STEPS_SDXL"); isSet {
		if intValue, err := strconv.Atoi(value); err == nil {
			defaults.StepsSDXL = intValue
		} else {
			defaults.StepsSDXL = 25
		}
	} else {
		defaults.StepsSDXL = 25
	}
	if value, isSet := os.LookupEnv("DEFAULT_CFG_SCALE"); isSet {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			defaults.CFGScale = floatValue
		} else {
			defaults.CFGScale = 7.0
		}
	} else {
		defaults.CFGScale = 7.0
	}
	if value, isSet := os.LookupEnv("ALLOWED_USER_IDS"); isSet {
		defaults.AllowedUserIDs = value
	}
	if value, isSet := os.LookupEnv("ALLOWED_GROUP_IDS"); isSet {
		defaults.AllowedGroupIDs = value
	}
	if value, isSet := os.LookupEnv("ADMIN_USER_IDS"); isSet {
		defaults.AdminUserIDs = value
	}
	if value, isSet := os.LookupEnv("PROCESS_TIMEOUT"); isSet {
		var err error
		if defaults.ProcessTimeout, err = time.ParseDuration(value); err != nil {
			defaults.ProcessTimeout = 15 * time.Minute
		}
	} else {
		defaults.ProcessTimeout = 15 * time.Minute
	}
	return
}
