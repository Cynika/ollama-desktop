package app

import (
	"github.com/hashicorp/go-version"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"net"
	"net/http"
	"net/url"
	"ollama-desktop/internal/config"
	"ollama-desktop/internal/log"
	olm "ollama-desktop/internal/ollama"
	"ollama-desktop/internal/ollama/api"
	"ollama-desktop/internal/ollama/cmd"
	ollama2 "ollama-desktop/internal/ollama/ollama"
	"ollama-desktop/internal/util"
	"os"
	gorun "runtime"
	"strings"
	"sync"
)

var ollama = Ollama{}

type Ollama struct {
	started       bool
	version       *version.Version
	lastVersion   *util.Item
	checkUpgraded bool
	lock          sync.Mutex
}

func (o *Ollama) Envs() []*OllamaEnvVar {
	envs := []*OllamaEnvVar{
		{"OLLAMA_DEBUG", cleanEnvValue("OLLAMA_DEBUG"), "Show additional debug information (e.g. OLLAMA_DEBUG=1)"},
		{"OLLAMA_FLASH_ATTENTION", cleanEnvValue("OLLAMA_FLASH_ATTENTION"), "Enabled flash attention"},
		{"OLLAMA_KV_CACHE_TYPE", cleanEnvValue("OLLAMA_KV_CACHE_TYPE"), "Quantization type for the K/V cache (default: f16)"},
		{"OLLAMA_GPU_OVERHEAD", cleanEnvValue("OLLAMA_GPU_OVERHEAD"), "Reserve a portion of VRAM per GPU (bytes)"},
		{"OLLAMA_HOST", cleanEnvValue("OLLAMA_HOST"), "IP Address for the ollama server (default 127.0.0.1:11434)"},
		{"OLLAMA_KEEP_ALIVE", cleanEnvValue("OLLAMA_KEEP_ALIVE"), "The duration that models stay loaded in memory (default \"5m\")"},
		{"OLLAMA_LLM_LIBRARY", cleanEnvValue("OLLAMA_LLM_LIBRARY"), "Set LLM library to bypass autodetection"},
		{"OLLAMA_LOAD_TIMEOUT", cleanEnvValue("OLLAMA_LOAD_TIMEOUT"), "How long to allow model loads to stall before giving up (default \"5m\")"},
		{"OLLAMA_MAX_LOADED_MODELS", cleanEnvValue("OLLAMA_MAX_LOADED_MODELS"), "Maximum number of loaded models per GPU"},
		{"OLLAMA_MAX_QUEUE", cleanEnvValue("OLLAMA_MAX_QUEUE"), "Maximum number of queued requests"},
		//{"OLLAMA_MAX_VRAM", cleanEnvValue(""), "Maximum VRAM"},
		{"OLLAMA_MODELS", cleanEnvValue("OLLAMA_MODELS"), "The path to the models directory"},
		{"OLLAMA_NOHISTORY", cleanEnvValue("OLLAMA_NOHISTORY"), "Do not preserve readline history"},
		{"OLLAMA_NOPRUNE", cleanEnvValue("OLLAMA_NOPRUNE"), "Do not prune model blobs on startup"},
		{"OLLAMA_NUM_PARALLEL", cleanEnvValue("OLLAMA_NUM_PARALLEL"), "Maximum number of parallel requests"},
		{"OLLAMA_ORIGINS", cleanEnvValue("OLLAMA_ORIGINS"), "A comma separated list of allowed origins"},
		//{"OLLAMA_RUNNERS_DIR", cleanEnvValue(""), "Location for runners"},
		{"OLLAMA_SCHED_SPREAD", cleanEnvValue("OLLAMA_SCHED_SPREAD"), "Always schedule model across all GPUs"},
		//{"OLLAMA_TMPDIR", cleanEnvValue(""), "Location for temporary files"},
		{"OLLAMA_MULTIUSER_CACHE", cleanEnvValue("OLLAMA_MULTIUSER_CACHE"), "Optimize prompt caching for multi-user scenarios"},

		// Informational
		{"HTTP_PROXY", cleanEnvValue("HTTP_PROXY"), "HTTP proxy"},
		{"HTTPS_PROXY", cleanEnvValue("HTTPS_PROXY"), "HTTPS proxy"},
		{"NO_PROXY", cleanEnvValue("NO_PROXY"), "No proxy"},
	}
	if gorun.GOOS != "windows" {
		// Windows environment variables are case-insensitive so there's no need to duplicate them
		envs = append(envs, &OllamaEnvVar{"http_proxy", cleanEnvValue("http_proxy"), "HTTP proxy"})
		envs = append(envs, &OllamaEnvVar{"https_proxy", cleanEnvValue("https_proxy"), "HTTPS proxy"})
		envs = append(envs, &OllamaEnvVar{"no_proxy", cleanEnvValue("no_proxy"), "No proxy"})
	}
	if gorun.GOOS != "darwin" {
		envs = append(envs, &OllamaEnvVar{"CUDA_VISIBLE_DEVICES", cleanEnvValue("CUDA_VISIBLE_DEVICES"), "Set which NVIDIA devices are visible"})
		envs = append(envs, &OllamaEnvVar{"HIP_VISIBLE_DEVICES", cleanEnvValue("HIP_VISIBLE_DEVICES"), "Set which AMD devices are visible"})
		envs = append(envs, &OllamaEnvVar{"ROCR_VISIBLE_DEVICES", cleanEnvValue("ROCR_VISIBLE_DEVICES"), "Set which AMD devices are visible"})
		envs = append(envs, &OllamaEnvVar{"GPU_DEVICE_ORDINAL", cleanEnvValue("GPU_DEVICE_ORDINAL"), "Set which AMD devices are visible"})
		envs = append(envs, &OllamaEnvVar{"HSA_OVERRIDE_GFX_VERSION", cleanEnvValue("HSA_OVERRIDE_GFX_VERSION"), "Override the gfx used for all detected AMD GPUs"})
		envs = append(envs, &OllamaEnvVar{"OLLAMA_INTEL_GPU", cleanEnvValue("OLLAMA_INTEL_GPU"), "Enable experimental Intel GPU detection"})
	}
	return envs
}

// Clean quotes and spaces from the value
func cleanEnvValue(key string) string {
	return strings.Trim(strings.TrimSpace(os.Getenv(key)), "\"'")
}

type OllamaEnvVar struct {
	Name        string
	Value       string
	Description string
}

func (o *Ollama) Version() (string, error) {
	return o.newApiClient().Version(app.ctx)
}

func (o *Ollama) Heartbeat() {
	o.lock.Lock()
	defer o.lock.Unlock()
	var installed, started bool
	client := o.newApiClient()
	started = client.Heartbeat(app.ctx) == nil
	if started != o.started {
		o.started = started
		o.version = nil
		o.checkUpgraded = false
	}

	if !started {
		installed, _ = cmd.CheckInstalled(app.ctx)
	} else {
		installed = true
	}
	if started && o.version == nil {
		current, _ := client.Version(app.ctx)
		if current != "" {
			ver, err := version.NewVersion(strings.ToLower(current))
			if err != nil {
				log.Warn().Err(err).Msg("get ollama version")
			} else {
				o.version = ver
			}
		}
	}
	if o.lastVersion == nil {
		client := createHttpClient()
		release := util.GithubRelease{Http: client}
		item, err := release.Last("ollama", "ollama")
		if err != nil {
			log.Warn().Err(err).Str("channel", release.Channel()).Msg("check ollama upgrade")
		} else if item != nil {
			log.Info().Str("channel", release.Channel()).Str("name", item.Name).Msg("check ollama upgrade")
			o.lastVersion = item
		}
	}
	upgrade := !o.checkUpgraded && o.lastVersion != nil && o.version != nil
	if upgrade {
		last, err := version.NewVersion(strings.ToLower(o.lastVersion.Name))
		if err != nil {
			log.Warn().Err(err).Msg("check ollama upgrade")
		} else {
			if last != nil {
				upgrade = last.GreaterThan(o.version)
			}
			o.checkUpgraded = true
		}

	}
	goos := gorun.GOOS
	current := ""
	if o.version != nil {
		current = o.version.String()
	}
	runtime.EventsEmit(app.ctx, "ollamaHeartbeat",
		installed, started, !started && installed && (goos == "windows" || goos == "darwin"),
		current, upgrade, o.lastVersion)
}

func (o *Ollama) Start() error {
	err := cmd.StartApp(app.ctx, o.newApiClient())
	if err != nil {
		log.Error().Err(err).Msg("start ollama app error")
		return err
	}
	o.Heartbeat()
	return nil
}

func (o *Ollama) List() (*olm.ListResponse, error) {
	resp, err := o.newApiClient().List(app.ctx)
	if err != nil {
		log.Error().Err(err).Msg("list ollama model error")
	}
	return resp, err
}

func (o *Ollama) ListRunning() (*olm.ProcessResponse, error) {
	resp, err := o.newApiClient().ListRunning(app.ctx)
	if err != nil {
		log.Error().Err(err).Msg("list ollama running model error")
	}
	return resp, err
}

func (o *Ollama) Delete(request *olm.DeleteRequest) error {
	err := o.newApiClient().Delete(app.ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("delete ollama model error")
	}
	return err
}

func (o *Ollama) Show(request *olm.ShowRequest) (*olm.ShowResponse, error) {
	log.Error().Any("request", request).Msg("Show")
	resp, err := o.newApiClient().Show(app.ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("show ollama model error")
	}
	return resp, err
}

func (o *Ollama) Pull(requestId string, request *olm.PullRequest) error {
	go func() {
		err := o.newApiClient().Pull(app.ctx, request, func(response olm.ProgressResponse) error {
			runtime.EventsEmit(app.ctx, requestId, response)
			return nil
		})
		if err != nil {
			log.Error().Err(err).Msg("pull ollama model error")
		}
	}()
	return nil
}

func (o *Ollama) Embeddings(request *olm.EmbeddingRequest) (*olm.EmbeddingResponse, error) {
	log.Error().Any("request", request).Msg("Embeddings")
	resp, err := o.newApiClient().Embeddings(app.ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("embeddings error")
	}
	return resp, err
}

func (o *Ollama) SearchOnline(request *olm.SearchRequest) ([]*olm.ModelInfo, error) {
	resp, err := o.newOllamaClient().Search(app.ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("search ollama model error")
	}
	return resp, err
}

func (o *Ollama) LibraryOnline(request *olm.LibraryRequest) ([]*olm.ModelInfo, error) {
	resp, err := o.newOllamaClient().Library(app.ctx, request)
	if err != nil {
		log.Error().Err(err).Msg("ollama library error")
	}
	return resp, err
}

func (o *Ollama) ModelInfoOnline(modelTag string) (*olm.ModelInfoResponse, error) {
	resp, err := o.newOllamaClient().ModelInfo(app.ctx, modelTag)
	if err != nil {
		log.Error().Err(err).Msg("ollama model info error")
	}
	return resp, err
}

func (o *Ollama) ModelTagsOnline(model string) (*olm.ModelTagsResponse, error) {
	resp, err := o.newOllamaClient().ModelTags(app.ctx, model)
	if err != nil {
		log.Error().Err(err).Msg("ollama model info error")
	}
	return resp, err
}

func (o *Ollama) newApiClient() *api.Client {
	ollamaHost := config.Config.Ollama.Host

	scheme, _ := configStore.getOrDefault(configOllamaScheme, ollamaHost.Scheme)
	host, _ := configStore.getOrDefault(configOllamaHost, ollamaHost.Host)
	port, _ := configStore.getOrDefault(configOllamaPort, ollamaHost.Port)

	return &api.Client{
		Base: &url.URL{
			Scheme: scheme,
			Host:   net.JoinHostPort(host, port),
		},
		Http: http.DefaultClient,
	}
}

func (o *Ollama) newOllamaClient() *ollama2.Client {
	base, _ := url.Parse("https://ollama.com")

	return &ollama2.Client{
		Base: base,
		Http: createHttpClient(),
	}
}
