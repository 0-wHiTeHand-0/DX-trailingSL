package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Darwin struct {
	Name       string `json:"name"`
	TrailingSL string `json:"trailingSL"`
}

type Config struct {
	AuthToken      string   `json:"authtoken"`
	ConsumerKey    string   `json:"consumerkey"`
	ConsumerSecret string   `json:"consumersecret"`
	RefreshToken   string   `json:"refreshtoken"`
	Darwins        []Darwin `json:"darwins"`
}

type InvestorAccount struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Threshold struct {
	Type    string  `json:"type"`
	OrderID int     `json:"orderId"`
	Amount  float32 `json:"amount"`
	Quote   float32 `json:"quote"`
}

type CurrentPosition struct {
	Pname      string      `json:"productname"`
	Thresholds []Threshold `json:"thresholds"`
	Cquote     float32     `json:"currentquote"`
}

// Send GET request and check authn token
func sendGet(url string, conf Config) string {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		os.Exit(1)
	}

	req.Header.Set("Authorization", "Bearer "+conf.AuthToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending GET request:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	var ret string
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		ret = string(bodyBytes)
	} else if resp.StatusCode == http.StatusUnauthorized {
		ret = "unauthorized"
	} else {
		ret = "unknown"
	}
	return ret
}

func saveondisk(conf Config, f string) {
	// Save the new config on disk
	file, err := json.MarshalIndent(conf, "", " ")
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		os.Exit(1)
	}
	err = os.WriteFile(f, file, 0600)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		os.Exit(1)
	}
	fmt.Println("New authentication tokens saved on disk!")
}

func refresh(oldconf Config, filename string) Config {
	// Send POST request
	client := &http.Client{}
	data := "grant_type=refresh_token&refresh_token=" + oldconf.RefreshToken
	req, err := http.NewRequest("POST", "https://api.darwinex.com/token", strings.NewReader(data))
	if err != nil {
		fmt.Println("Error creating request:", err)
		os.Exit(1)
	}

	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(oldconf.ConsumerKey+":"+oldconf.ConsumerSecret)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error refreshing the authn token: ", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error refreshing the authn token. Got status code", resp.Status)
		fmt.Println("Please refresh the tokens manually from the Darwinex website, and try again.")
		os.Exit(1)
	}

	// Parse the JSON response
	type Refresh struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	var newref Refresh
	err = json.NewDecoder(resp.Body).Decode(&newref)
	if err != nil {
		fmt.Println("Error parsing JSON response of the refresh query:", err)
		os.Exit(1)
	}

	oldconf.AuthToken = newref.AccessToken
	oldconf.RefreshToken = newref.RefreshToken

	saveondisk(oldconf, filename)

	return oldconf
}

func sendPut(wg *sync.WaitGroup, url string, darname string, newstop float64, amount float32, conf Config) {
	defer wg.Done()
	client := &http.Client{}
	data := `{"amount":` + strconv.FormatFloat(float64(amount), 'f', 2, 32) + `,"quote":` + strconv.FormatFloat(newstop, 'f', 2, 32) + `}`
	req, err := http.NewRequest("PUT", url, strings.NewReader(data))
	if err != nil {
		fmt.Println("Error creating PUT request:", err)
		os.Exit(1)
	}

	req.Header.Set("Authorization", "Bearer "+conf.AuthToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending PUT request:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		fmt.Println("Trailing stop-loss order updated for " + darname + ". New stop-loss value: " + strconv.FormatFloat(newstop, 'f', 2, 32))
	} else {
		fmt.Println("Unknown error while updating the trailing stop-loss order for " + darname + ". Got status code " + resp.Status)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: ./test config.json")
		os.Exit(1)
	}
	filename := os.Args[1]

	// Read the JSON file
	file, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error reading config file:", err)
		os.Exit(1)
	}

	// Parse the JSON data into a Config struct
	var config Config
	err = json.Unmarshal(file, &config)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		os.Exit(1)
	}

	// Sanitization process
	for _, darwin := range config.Darwins {
		regex := regexp.MustCompile(`^\d+(\.\d+)?%?$`)
		if !regex.MatchString(darwin.TrailingSL) {
			fmt.Println("Error: trailingSL must be a number or a percentage, e.g. 46.5 or 2.5%")
			os.Exit(1)
		}
	}

	// Check if config is empty
	if config.AuthToken == "" || config.ConsumerKey == "" || config.ConsumerSecret == "" || config.RefreshToken == "" || len(config.Darwins) == 0 {
		fmt.Println("Unexpected JSON format or empty value found")
		os.Exit(1)
	}

	// Check if any Darwin struct is empty
	for _, darwin := range config.Darwins {
		if darwin.Name == "" || darwin.TrailingSL == "" {
			fmt.Println("Unexpected JSON format or empty value found")
			os.Exit(1)
		}
	}

	invResp := sendGet("https://api.darwinex.com/investoraccountinfo/2.0/investoraccounts", config)
	if invResp == "unknown" {
		fmt.Println("Unknown error while getting the investor ID")
		os.Exit(1)
	} else if invResp == "unauthorized" {
		fmt.Println("Expired authentication token. Refreshing...")
		config = refresh(config, filename)
		invResp = sendGet("https://api.darwinex.com/investoraccountinfo/2.0/investoraccounts", config)
	}
	if invResp == "unknown" || invResp == "unauthorized" {
		fmt.Println("Unknown error getting the investorID. Can not proceed!")
		os.Exit(1)
	}

	// Parse the JSON response
	var investorAccounts []InvestorAccount
	err = json.NewDecoder(strings.NewReader(invResp)).Decode(&investorAccounts)
	if err != nil {
		fmt.Println("Error parsing JSON response for the investorID query:", err)
		os.Exit(1)
	}

	fmt.Println("I will process the stop-loss orders just for the first investor ID (", investorAccounts[0].ID, ") and the first account (", investorAccounts[0].Name, ")")

	posResp := sendGet("https://api.darwinex.com/investoraccountinfo/2.0/investoraccounts/"+strconv.Itoa(investorAccounts[0].ID)+"/currentpositions", config)
	if posResp == "unknown" || posResp == "unauthorized" {
		fmt.Println("Unknown error getting the investorID. Can not proceed!")
		os.Exit(1)
	}

	// Parse the JSON response
	var positions []CurrentPosition
	err = json.NewDecoder(strings.NewReader(posResp)).Decode(&positions)
	if err != nil {
		fmt.Println("Error parsing JSON response for the currentpositions query:", err)
		os.Exit(1)
	}
	wg := new(sync.WaitGroup)
	flag1 := false
	flag2 := false
	for _, position := range positions {
		posname := position.Pname
		if strings.Contains(position.Pname, ".") {
			posname = strings.Split(position.Pname, ".")[0]
		}
		for _, darwin := range config.Darwins {
			if posname == darwin.Name {
				flag3 := false
				for _, threshold := range position.Thresholds {
					if threshold.Type == "STOP_LOSS" {
						flag1 = true
						flag3 = true
						var magicnumber float64
						if strings.Contains(darwin.TrailingSL, "%") {
							magicnumber, err = strconv.ParseFloat(strings.ReplaceAll(darwin.TrailingSL, "%", ""), 32)
							if err != nil {
								fmt.Println("Error parsing the trailingSL value:", err)
								os.Exit(1)
							}
							magicnumber = (magicnumber / 100) * float64(position.Cquote)
						} else {
							magicnumber, err = strconv.ParseFloat(darwin.TrailingSL, 32)
							if err != nil {
								fmt.Println("Error parsing the trailingSL value:", err)
								os.Exit(1)
							}
						}

						if magicnumber+0.005 < float64(position.Cquote-threshold.Quote) {
							wg.Add(1)
							go sendPut(wg, "https://api.darwinex.com/trading/1.1/investoraccounts/"+strconv.Itoa(investorAccounts[0].ID)+"/conditionalorders/"+strconv.Itoa(threshold.OrderID), darwin.Name, float64(position.Cquote)-magicnumber, threshold.Amount, config)
							flag2 = true
						}
						break
					}
				}
				if !flag3 {
					fmt.Println("WARNING: No stop-loss found for", darwin.Name, "so I can not update it. Please set a stop-loss order manually in the Darwinex website.")
				}
				break
			}
		}
	}
	if !flag1 {
		fmt.Println("WARNING: No stop-loss order found for any of the Darwins in the config file.")
	}
	if !flag2 {
		fmt.Println("No stop-loss needs update!")
	}
	wg.Wait()
}
