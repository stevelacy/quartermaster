package manager

import (
	"encoding/json"
	"errors"
	"net/http"
)

func HandleAuth(w http.ResponseWriter, r *http.Request) (PostRequest, error) {
	w.Header().Set("Content-Type", "application/json")
	var response PostRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&response)
	if err != nil {
		w.WriteHeader(400)
		payload := PostErrorResponse{Success: false, Error: err.Error(), Code: 400}
		_ = json.NewEncoder(w).Encode(payload)
		return PostRequest{}, err
	}
	if response.Token != root_token {
		err := errors.New("Unauthorized")
		w.WriteHeader(401)
		payload := PostErrorResponse{Success: false, Error: err.Error(), Code: 400}
		_ = json.NewEncoder(w).Encode(payload)
		return PostRequest{}, err
	}
	return response, nil
}
