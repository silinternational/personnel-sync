package freshworks

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"log/syslog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Jeffail/gabs/v2"

	internal "github.com/silinternational/personnel-sync/v5/internal"
)

const (
	DefaultBatchSize         = 10
	DefaultBatchDelaySeconds = 3
)

type Freshteam struct {
	BaseURL           string
	Token             string
	CompareAttribute  string
	UserAgent         string
	BatchSize         int
	BatchDelaySeconds int
	destinationConfig internal.DestinationConfig
}

// NewFreshteamSource unmarshals the sourceConfig's ExtraJson into a Freshteam struct
func NewFreshteamSource(sourceConfig internal.SourceConfig) (internal.Source, error) {
	var freshteam Freshteam
	// Unmarshal ExtraJSON into Freshteam struct
	err := json.Unmarshal(sourceConfig.ExtraJSON, &freshteam)
	if err != nil {
		return &Freshteam{}, err
	}

	freshteam.setDefaults()

	return &freshteam, nil
}

// NewFreshteamDestination unmarshals the destinationConfig's ExtraJson into a Freshteam struct
func NewFreshteamDestination(destinationConfig internal.DestinationConfig) (internal.Destination, error) {
	var freshteam Freshteam
	// Unmarshal ExtraJSON into GoogleGroupsConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &freshteam)
	if err != nil {
		return &Freshteam{}, err
	}

	freshteam.setDefaults()
	freshteam.destinationConfig = destinationConfig

	return &freshteam, nil
}

func (r *Freshteam) ForSet(syncSetJson json.RawMessage) error {
	// unused in Freshteam
	return nil
}

// ListUsers makes an http request and uses the response to populate
// and return a slice of Person instances
func (r *Freshteam) ListUsers(desiredAttrs []string) ([]internal.Person, error) {
	errLog := make(chan string, 1000)
	people := make(chan internal.Person, 20000)

	r.listUsersForPath(desiredAttrs, "/employees", people, errLog)

	close(people)
	close(errLog)

	if len(errLog) > 0 {
		var errs []string
		for msg := range errLog {
			errs = append(errs, msg)
		}
		return []internal.Person{}, fmt.Errorf("errors listing Freshteam users: %s", strings.Join(errs, ","))
	}

	var results []internal.Person

	for person := range people {
		results = append(results, person)
	}

	return results, nil
}

func (r *Freshteam) ApplyChangeSet(changes internal.ChangeSet, eventLog chan<- internal.EventLogItem) internal.ChangeResults {
	var results internal.ChangeResults
	var wg sync.WaitGroup

	batchTimer := internal.NewBatchTimer(r.BatchSize, r.BatchDelaySeconds)

	if r.destinationConfig.DisableAdd {
		log.Println("Creation is disabled.")
	} else {
		for _, toCreate := range changes.Create {
			wg.Add(1)
			go r.addContact(toCreate, &results.Created, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	if r.destinationConfig.DisableUpdate {
		log.Println("Update is disabled.")
	} else {
		log.Println("Freshteam does not yet support update.")
	}

	if r.destinationConfig.DisableDelete {
		log.Println("Delete is disabled.")
	} else {
		log.Println("Freshteam does not yet support delete.")
	}

	wg.Wait()

	return results
}

func (r *Freshteam) listUsersForPath(
	desiredAttrs []string,
	path string,
	people chan<- internal.Person,
	errLog chan<- string) {

	client := &http.Client{}
	apiURL := fmt.Sprintf("%s%s", r.BaseURL, path)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		log.Println(err)
		errLog <- err.Error()
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.Token))

	resp, err := client.Do(req)
	if err != nil {
		errLog <- "error issuing http request, " + err.Error()
		return
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errLog <- "error reading response body: " + err.Error()
		return
	}

	if resp.StatusCode > 299 {
		msg := fmt.Sprintf("response status code: %d response body: %s", resp.StatusCode, bodyText)
		log.Print(msg)
		errLog <- msg
		return
	}

	// TODO: this probably doesn't need gabs
	jsonParsed, err := gabs.ParseJSON(bodyText)
	if err != nil {
		log.Printf("error parsing json results: %s", err.Error())
		log.Printf("response body: %s", string(bodyText))
		errLog <- err.Error()
		return
	}
	peopleList := jsonParsed.Children()

	results := getPersonsFromResults(peopleList, r.CompareAttribute, desiredAttrs)

	for _, person := range results {
		people <- person
	}
}

func getPersonsFromResults(peopleList []*gabs.Container, compareAttr string, desiredAttrs []string) []internal.Person {
	sourcePeople := make([]internal.Person, 0)

	// TODO: can this be either simplified or extracted to a utility function in `internal`
	for _, person := range peopleList {
		peep := internal.Person{
			Attributes: map[string]string{},
		}

		for _, sourceKey := range desiredAttrs {
			if !person.ExistsP(sourceKey) {
				continue
			}

			val := person.Path(sourceKey).Data()
			if val == nil {
				continue
			}

			switch v := val.(type) {
			case []interface{}:
				if len(val.([]interface{})) > 0 {
					firstValue := val.([]interface{})[0]
					if firstValue == nil {
						continue
					}

					var ok bool
					if peep.Attributes[sourceKey], ok = firstValue.(string); !ok {
						log.Printf("not a string, sourceKey=%s: %+v, type %T", sourceKey, firstValue, firstValue)
					}
				}
			default:
				peep.Attributes[sourceKey] = fmt.Sprintf("%v", v)
			}

			if sourceKey == compareAttr {
				peep.CompareValue = person.Path(sourceKey).Data().(string)
			}
		}

		// If person is missing a compare value, do not append them to list
		if peep.CompareValue == "" {
			continue
		}

		sourcePeople = append(sourcePeople, peep)
	}

	return sourcePeople
}

func (r *Freshteam) setDefaults() {
	// TODO: this looks like a candidate for a helper in `internal`
	if r.BatchSize <= 0 {
		r.BatchSize = DefaultBatchSize
	}
	if r.BatchDelaySeconds <= 0 {
		r.BatchDelaySeconds = DefaultBatchDelaySeconds
	}
	if r.UserAgent == "" {
		r.UserAgent = "personnel-sync"
	}
}

func (r *Freshteam) addContact(p internal.Person, n *uint64, wg *sync.WaitGroup, eventLog chan<- internal.EventLogItem) {
	defer wg.Done()

	apiURL := fmt.Sprintf("%s%s", r.BaseURL, "/employees")
	headers := map[string]string{"Content-Type": "application/json"}
	responseBody, err := r.httpRequest(http.MethodPost, apiURL, attributesToJSON(p.Attributes), headers)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level: syslog.LOG_ERR,
			Message: fmt.Sprintf("addContact %s httpRequest error %s, response: %s", p.CompareValue, err,
				responseBody),
		}
		return
	}

	eventLog <- internal.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "AddContact " + p.CompareValue,
	}

	atomic.AddUint64(n, 1)
}

func attributesToJSON(attr map[string]string) string {
	jsonObj := gabs.New()
	for field, value := range attr {
		if _, err := jsonObj.SetP(value, field); err != nil {
			log.Printf("error setting field %s for Freshteam API add, %s", field, err)
		}
	}
	if _, err := jsonObj.SetP([]int{6000209129}, "role_ids"); err != nil {
		log.Printf("error setting role_ids for Freshteam API, %s", err)
	}
	return jsonObj.String()
}

func (r *Freshteam) httpRequest(verb, url, body string, headers map[string]string) (string, error) {
	var req *http.Request
	var err error
	if body == "" {
		req, err = http.NewRequest(verb, url, nil)
	} else {
		req, err = http.NewRequest(verb, url, strings.NewReader(body))
	}
	if err != nil {
		return "", err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", r.UserAgent)

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.Token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read http response body: %s", err)
	}
	bodyString := string(bodyBytes)

	if resp.StatusCode >= 400 {
		return bodyString, errors.New(resp.Status)
	}

	return bodyString, nil
}
