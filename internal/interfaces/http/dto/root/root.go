package root

type RootResponse struct {
	AppName string `json:"app_name"`
	AppVersion string `json:"app_version"`
	AppEnv string `json:"app_env"`
	Status      string `json:"status"`

}
