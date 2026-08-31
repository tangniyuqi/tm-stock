package quant

import (
	"encoding/json"
	"io"
	"net/http"
)

type UpdateService struct{}

// GetClientUpdate proxies the request to fetch client update information
func (s *UpdateService) GetClientUpdate() (map[string]interface{}, error) {
	url := "https://go.noooya.com/uploads/client/update.json"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
