package ctftime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"math/rand"
	"models"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/charmbracelet/log"
)

// BaseURL is the CTFtime API root. It is a variable so tests can point it at a local server.
var BaseURL = "https://ctftime.org/api/v1"

// httpClient bounds every CTFtime call: events are handled on the gateway
// goroutine, so a request that never returns would freeze the whole bot.
var httpClient = &http.Client{Timeout: 20 * time.Second}

var UserAgentList = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.5 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.53 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Windows; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.114 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/603.3.8 (KHTML, like Gecko) Version/10.1.2 Safari/603.3.8",
	"Mozilla/5.0 (Windows NT 10.0; Windows; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.114 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.53 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Windows; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.114 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.53 Safari/537.36",
}

func pickRandomAgent() string {
	return UserAgentList[rand.Intn(len(UserAgentList))]
}

func createRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err == nil {
		req.Header.Set("User-Agent", pickRandomAgent())
	}
	if err != nil {
		log.Error("Error creating request:", "err", err)
		return nil, err
	}
	return req, nil
}

func GetLogo(url string) ([]byte, string, error) {
	log.Debug("Fetching logo from URL:", "url", url)
	req, err := createRequest(url)
	if err != nil {
		log.Error("Error creating request for logo:", "err", err)
		return nil, "", err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error("Error sending request for logo:", "err", err)
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch logo: status %d", resp.StatusCode)
	}

	const maxSize = 5 << 20 // 5 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		log.Error("Error reading logo response body:", "err", err)
		return nil, "", err
	}

	log.Debug("Logo data fetched, size:", "size", len(data))

	// DecodeConfig only reads the header, so a hostile image can't exhaust memory.
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		log.Error("Error decoding image data:", "err", err)
		return nil, "", err
	}

	log.Debug("Successfully fetched and decoded logo", "format", format)
	return data, format, nil
}

func GetCTFInfo(ctftimeID int) (Event, error) {
	idParsed := strconv.Itoa(ctftimeID)
	url := BaseURL + "/events/" + idParsed + "/"
	log.Debug("Fetching CTF info from URL:", "url", url)
	req, err := createRequest(url)
	if err != nil {
		log.Error("Error creating request for CTF info:", "err", err)
		return Event{}, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error("Error sending request:", "err", err)
		return Event{}, err
	}
	defer resp.Body.Close()

	var parsedJSON Event
	if resp.StatusCode != http.StatusOK {
		return Event{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&parsedJSON)
	if err != nil {
		return Event{}, err
	}

	return parsedJSON, nil
}

func GetCTFs(limit int, onsite bool) ([]Event, error) {
	url := BaseURL + "/events/" + "?limit=" + strconv.Itoa(limit)
	log.Debug("Fetching CTFs from URL:", "url", url)
	req, err := createRequest(url)
	if err != nil {
		log.Error("Error creating request for CTFs:", "err", err)
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error("Error sending request:", "err", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Debug("Received response for CTFs", "status_code", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		log.Error("Non-200 status code received for CTFs", "status_code", resp.StatusCode)
		return nil, fmt.Errorf("fetch CTFs: status %d", resp.StatusCode)
	}

	var parsedJSON []Event
	jsonResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Error reading response body for CTFs:", "err", err)
		return nil, err
	}

	err = json.Unmarshal(jsonResponse, &parsedJSON)
	if err != nil {
		log.Error("Error unmarshalling CTFs JSON:", "err", err)
		return nil, err
	}

	if !onsite {
		for i := 0; i < len(parsedJSON); i++ {
			if parsedJSON[i].Onsite {
				log.Debug("Skipping non-onsite CTF", "ctf_id", parsedJSON[i].ID, "title", parsedJSON[i].Title)
				parsedJSON = append(parsedJSON[:i], parsedJSON[i+1:]...)
				i--
			}
		}
	}
	log.Debug("Returning parsed CTFs", "count", len(parsedJSON))
	return parsedJSON, nil
}

type ResultScore struct {
	TeamID int64  `json:"team_id"`
	Place  int    `json:"place"`
	Points string `json:"points"`
	Solves int    `json:"solves"`
}

func GetResultsInfo(ctftimeID int64, year int, teamID int64) (*models.Report, error) {
	log.Debug("Getting results for event with ID", "ctftime_id", ctftimeID)
	url := fmt.Sprintf("%s/results/%d/", BaseURL, year)
	log.Debug("Fetching CTF results from URL:", "url", url)

	req, err := createRequest(url)
	if err != nil {
		log.Error("Error creating request for results:", "err", err)
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error("Error sending request for results:", "err", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error("failed to retrieve CTF results information. Status code:", "status", resp.StatusCode)
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var responseData map[string]struct {
		Scores []ResultScore `json:"scores"`
	}
	err = json.NewDecoder(resp.Body).Decode(&responseData)
	if err != nil {
		log.Error("Error decoding results response:", "err", err)
		return nil, err
	}

	log.Debug("Looking for event ID in results", "ctftime_id", ctftimeID)
	log.Debug("Looking for team ID in results", "team_id", teamID)

	eventKey := strconv.FormatInt(ctftimeID, 10)
	eventData, ok := responseData[eventKey]
	if !ok {
		log.Error("Event ID not found in results response", "ctftime_id", ctftimeID)
		return nil, nil
	}

	for _, result := range eventData.Scores {
		if result.TeamID == teamID {
			scoreFloat, err := strconv.ParseFloat(strings.TrimSpace(result.Points), 64)
			if err != nil {
				log.Error("Error parsing score to float:", "err", err)
				return nil, err
			}
			scoreInt := int(scoreFloat)
			report := &models.Report{
				Place:  result.Place,
				Score:  scoreInt,
				Solves: result.Solves,
			}
			return report, nil
		}
	}

	log.Error("Team ID not found in results", "team_id", teamID)
	return nil, nil
}
