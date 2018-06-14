package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	var address, vaultAddress, vaultToken string
	flag.StringVar(&address, "address", "127.0.0.1:8899", "listen address")
	flag.StringVar(&vaultAddress, "vault-address", "127.0.0.1:8200", "vault address")
	flag.StringVar(&vaultToken, "vault-token", "", "vault token")
	flag.Parse()

	h := handlers{
		VaultAddress: vaultAddress,
		VaultToken:   vaultToken,
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/data/{id}", h.GetDataHandler).Methods("GET")
	router.HandleFunc("/api/v1/data/{id}", h.SetDataHandler).Methods("POST")

	fmt.Println("Starting vaulthub on " + address)
	log.Fatal(http.ListenAndServe(address, router))
}

type handlers struct {
	VaultAddress string
	VaultToken   string
}

func (h *handlers) GetDataHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	secret, err := getSecret(id, h.VaultAddress, h.VaultToken)
	if err != nil {
		log.Fatal(err)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, secret)
}

func (h *handlers) SetDataHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	var data struct {
		Value string
	}
	if err := json.Unmarshal(body, &data); err != nil {
		fmt.Println("Failed to parse JSON: " + string(body))
		log.Fatal(err)
	}

	if err := setSecret(id, data.Value, h.VaultAddress, h.VaultToken); err != nil {
		log.Fatal(err)
	}

	w.WriteHeader(http.StatusOK)
}

func getSecret(id, address, token string) (string, error) {
	url := address + "/v1/secret/data/" + id
	resp, err := vaultRequest("GET", url, nil, token)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var secretResponse struct {
		Data struct {
			Data struct {
				Value string
			}
		}
		Errors []string
	}
	if err := json.Unmarshal(body, &secretResponse); err != nil {
		fmt.Println("Failed to parse JSON: " + string(body))
		log.Fatal(err)
	}
	if len(secretResponse.Errors) > 0 {
		fmt.Println("Got error(s): " + strings.Join(secretResponse.Errors, ", "))
		log.Fatal(nil)
	}
	return secretResponse.Data.Data.Value, nil
}

func setSecret(id, value, address, token string) error {
	url := address + "/v1/secret/data/" + id
	body := strings.NewReader(fmt.Sprintf(`{"data": {"value": "%s"}}`, value))
	resp, err := vaultRequest("POST", url, body, token)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func vaultRequest(method, url string, body io.Reader, token string) (*http.Response, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("X-Vault-Token", token)
	return client.Do(req)
}
