package sdapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/kanootoko/stable-diffusion-telegram-bot/internal/consts"
)

const versionCheckTimeout = time.Second * 10

func versionCheck(ctx context.Context, stableDiffusionApiHost string) (latestVersion, currentVersion string, err error) {
	// AUTOMATIC1111 repository latest tag
	httpClient := http.Client{}
	client := github.NewClient(&httpClient)

	release, _, err := client.Repositories.GetLatestRelease(ctx, "AUTOMATIC1111", "stable-diffusion-webui")
	if err != nil {
		return "", "", fmt.Errorf("getting latest stable diffusion version: %w", err)
	}
	latestVersion = release.GetTagName()

	// Stable Diffusion API version available

	var request *http.Request
	var endpoint string
	endpoint, err = url.JoinPath(stableDiffusionApiHost, "/internal/sysinfo")
	if err != nil {
		return
	}
	request, err = http.NewRequestWithContext(
		ctx,
		"GET",
		endpoint,
		nil,
	)
	if err != nil {
		return
	}

	var resp *http.Response
	resp, err = httpClient.Do(request)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var sysInfoPart struct {
		Version string `json:"Version"`
	}
	err = json.Unmarshal(bodyBytes, &sysInfoPart)
	if err != nil {
		return
	}
	currentVersion = sysInfoPart.Version

	return latestVersion, currentVersion, nil
}

func VersionCheckGetStr(ctx context.Context, stableDiffusionApiHost string) (res string, updateNeededOrError bool) {
	verCheckCtx, verCheckCtxCancel := context.WithTimeout(ctx, versionCheckTimeout)
	defer verCheckCtxCancel()

	var latestVersion, currentVersion string
	var err error
	if latestVersion, currentVersion, err = versionCheck(verCheckCtx, stableDiffusionApiHost); err != nil {
		return consts.ErrorStr + ": " + err.Error(), true
	}

	updateNeededOrError = currentVersion != latestVersion
	res = "Stable Diffusion WebUI version: " + currentVersion
	if updateNeededOrError {
		res = "📢 " + res + " 📢 Update needed! Latest version is " + latestVersion + " 📢"
	} else {
		res += " (up to date)"
	}
	return
}
