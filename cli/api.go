package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var apiClient = &http.Client{Timeout: 15 * time.Second}

type apiTeam struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func createTeamViaAPI(cfg CliksConfig, name string, deletePassword string) (apiTeam, error) {
	var out struct {
		Team  apiTeam `json:"team"`
		Error string  `json:"error"`
	}
	err := apiJSON("POST", strings.TrimRight(cfg.APIURL, "/")+"/api/teams", map[string]string{
		"name":           name,
		"deletePassword": deletePassword,
	}, &out)
	if err != nil {
		return apiTeam{}, err
	}
	if out.Error != "" {
		return apiTeam{}, errors.New(out.Error)
	}
	return out.Team, nil
}

func getTeamViaAPI(cfg CliksConfig, code string) (apiTeam, error) {
	var out struct {
		Team  apiTeam `json:"team"`
		Error string  `json:"error"`
	}
	err := apiJSON("GET", strings.TrimRight(cfg.APIURL, "/")+"/api/teams/"+strings.ToUpper(code), nil, &out)
	if err != nil {
		return apiTeam{}, err
	}
	if out.Error != "" {
		return apiTeam{}, errors.New(out.Error)
	}
	return out.Team, nil
}

func deleteTeamViaAPI(cfg CliksConfig, code string, deletePassword string) error {
	var out struct {
		Error string `json:"error"`
	}
	err := apiJSON("DELETE", strings.TrimRight(cfg.APIURL, "/")+"/api/teams/"+strings.ToUpper(code), map[string]string{
		"deletePassword": deletePassword,
	}, &out)
	if err != nil {
		return err
	}
	if out.Error != "" {
		return errors.New(out.Error)
	}
	return nil
}

func apiJSON(method string, url string, input any, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := apiClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, output)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var errorBody struct {
			Error string `json:"error"`
		}
		if len(data) > 0 {
			_ = json.Unmarshal(data, &errorBody)
		}
		if errorBody.Error != "" {
			return errors.New(errorBody.Error)
		}
		return fmt.Errorf("server returned %s", response.Status)
	}
	return nil
}
