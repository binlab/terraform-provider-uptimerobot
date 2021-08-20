package uptimerobotapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/logging"
)

func New(apiKey string) UptimeRobotApiClient {
	return UptimeRobotApiClient{apiKey}
}

type UptimeRobotApiClient struct {
	apiKey string
}

func (client UptimeRobotApiClient) MakeCall(
	endpoint string,
	params string,
) (map[string]interface{}, error) {
	log.Printf("[DEBUG] Making request to: %#v", endpoint)

	url := "https://api.uptimerobot.com/v2/" + endpoint

	c := &http.Client{
		Transport: &http.Transport{},
	}

	c.Transport = logging.NewTransport("UptimeRobot", c.Transport)

	var res *http.Response

	payload := fmt.Sprintf("api_key=%s&format=json&%s", client.apiKey, params)

	for {
		req, err := http.NewRequest("POST", url, strings.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("contructing request: %w", err)
		}

		req.Header.Add("cache-control", "no-cache")
		req.Header.Add("content-type", "application/x-www-form-urlencoded")

		resCandidate, err := c.Do(req)
		if err != nil {
			return nil, fmt.Errorf("performing API request: %w", err)
		}

		switch resCandidate.StatusCode {
		case http.StatusTooManyRequests:
			waitForRetryAfter(resCandidate)
		default:
			res = resCandidate
			break
		}
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		return nil, fmt.Errorf("decoding response body %q: %w", string(body), err)
	}

	if result["stat"] != "ok" {
		message, _ := json.Marshal(result["error"])
		return nil, fmt.Errorf("got error from UptimeRobot: %s", string(message))
	}

	return result, nil
}

func waitForRetryAfter(res *http.Response) {
	retryAfterRaws, ok := res.Header["Retry-After"]
	if !ok || len(retryAfterRaws) > 1 {
		log.Printf("[DEBUG] Retry-After header is missing, waiting 1 second for next request attempt")
		retryAfterRaws = []string{"1"}
	}

	waitTime, err := time.ParseDuration(retryAfterRaws[0] + "s")
	if err != nil {
		log.Printf("[DEBUG] Parsing %q as Retry-After header value in seconds, waiting 1 second for next request: %v", retryAfterRaws, err)
		waitTime = time.Second
	}

	log.Printf("[DEBUG] Rate limit exceeded, waiting %d seconds to send next request", waitTime.Seconds())
	time.Sleep(waitTime)
}
