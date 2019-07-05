package googledest

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	personnel_sync "github.com/silinternational/personnel-sync"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
)

type GoogleContactsConfig struct {
	DelegatedAdminEmail string
	Domain              string
	GoogleAuth          GoogleAuth
}

type GoogleContacts struct {
	DestinationConfig    personnel_sync.DestinationConfig
	GoogleContactsConfig GoogleContactsConfig
	BatchSizePerMinute   int
	Client               http.Client
}

type Entries struct {
	XMLName xml.Name  `xml:"feed"`
	Entries []Contact `xml:"entry"`
}

type Contact struct {
	XMLName xml.Name `xml:"entry"`
	ID      string   `xml:"id"`
	Title   string   `xml:"title"`
	Name    Name     `xml:"name"`
	Emails  []Email  `xml:"email"`
}

type Email struct {
	XMLName xml.Name `xml:"email"`
	Address string   `xml:"address,attr"`
	Primary bool     `xml:"primary,attr"`
}

type Name struct {
	XMLName    xml.Name `xml:"name"`
	FullName   string   `xml:"fullName"`
	GivenName  string   `xml:"givenName"`
	FamilyName string   `xml:"familyName"`
}

func NewGoogleContactsDestination(destinationConfig personnel_sync.DestinationConfig) (personnel_sync.Destination, error) {
	var googleContacts GoogleContacts
	// Unmarshal ExtraJSON into GoogleContactsConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &googleContacts.GoogleContactsConfig)
	if err != nil {
		return &GoogleContacts{}, err
	}

	// Defaults
	if googleContacts.BatchSizePerMinute <= 0 {
		googleContacts.BatchSizePerMinute = DefaultBatchSizePerMinute
	}

	// Initialize Client object
	err = googleContacts.initGoogleClient()
	if err != nil {
		return &GoogleContacts{}, err
	}

	return &googleContacts, nil
}

func (g *GoogleContacts) GetIDField() string {
	return "ID"
}

func (g *GoogleContacts) ForSet(syncSetJson json.RawMessage) error {
	// sync sets not implemented for this destination
	return nil
}

func (g *GoogleContacts) httpRequest(verb string, url string, body string) (string, error) {
	var req *http.Request
	var err error
	if body == "" {
		req, err = http.NewRequest(verb, url, nil)
	} else {
		req, err = http.NewRequest(verb, url, bytes.NewBuffer([]byte(body)))
	}
	if err != nil {
		return "", err
	}

	req.Header.Set("GData-Version", "3.0")
	req.Header.Set("User-Agent", "personnel-sync")
	req.Header.Set("Content-Type", "application/atom+xml")

	resp, err := g.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	bodyString := string(bodyBytes)
	log.Println(bodyString)
	log.Println(resp.Status)

	if resp.StatusCode >= 400 {
		return bodyString, errors.New(resp.Status)
	}

	return bodyString, nil
}

func (g *GoogleContacts) ListUsers() ([]personnel_sync.Person, error) {
	href := "https://www.google.com/m8/feeds/contacts/" + g.GoogleContactsConfig.Domain + "/full"
	body, err := g.httpRequest("GET", href, "")
	if err != nil {
		return []personnel_sync.Person{}, fmt.Errorf("failed to retrieve user list: %s", err)
	}

	var parsed Entries

	err = xml.Unmarshal([]byte(body), &parsed)
	if err != nil {
		return []personnel_sync.Person{}, fmt.Errorf("failed to parse xml for user list: %s", err)
	}

	persons := make([]personnel_sync.Person, len(parsed.Entries))
	for i := 0; i < len(parsed.Entries); i++ {
		var primaryEmail string
		for j, e := range parsed.Entries[i].Emails {
			if e.Primary {
				primaryEmail = parsed.Entries[i].Emails[j].Address
				break
			}
		}

		persons[i] = personnel_sync.Person{
			CompareValue: primaryEmail,
			ID:           parsed.Entries[i].ID,
			Attributes: map[string]string{
				"email":    primaryEmail,
				"fullName": parsed.Entries[i].Title,
			},
		}
	}

	fmt.Println(parsed.Entries[0].Title)

	return persons, nil
}

func (g *GoogleContacts) ApplyChangeSet(
	changes personnel_sync.ChangeSet,
	eventLog chan<- personnel_sync.EventLogItem) personnel_sync.ChangeResults {

	var results personnel_sync.ChangeResults
	var wg sync.WaitGroup

	// One minute per batch
	batchTimer := personnel_sync.NewBatchTimer(g.BatchSizePerMinute, int(60))

	for _, cp := range changes.Create {
		wg.Add(1)
		go g.addContact(cp, &results.Created, &wg, eventLog)
		batchTimer.WaitOnBatch()
	}

	wg.Wait()

	return results
}

func (g *GoogleContacts) addContact(
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	// href := "https://www.google.com/m8/feeds/contacts/default/full"
	href := "https://www.google.com/m8/feeds/contacts/" + g.GoogleContactsConfig.Domain + "/full"
	bodyTemplate := `
	   	<atom:entry xmlns:atom='http://www.w3.org/2005/Atom'
	       xmlns:gd='http://schemas.google.com/g/2005'>
	     <atom:category scheme='http://schemas.google.com/g/2005#kind'
	       term='http://schemas.google.com/contact/2008#contact' />
	     <gd:name>
	        <gd:fullName>%s</gd:fullName>
	     </gd:name>
	     <gd:email rel='http://schemas.google.com/g/2005#work'
	       primary='true'
		   address='%s'/>
	   </atom:entry>
	`

	body := fmt.Sprintf(bodyTemplate, person.Attributes["fullName"], person.Attributes["email"])

	_, err := g.httpRequest("POST", href, body)
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("unable to insert %s in Google contacts: %s", person.CompareValue, err.Error())}
		return
	}

	eventLog <- personnel_sync.EventLogItem{
		Event:   "AddContact",
		Message: person.CompareValue,
	}

	atomic.AddUint64(counter, 1)
}

// initGoogleClent creates an http Client and adds a JWT config that has the required OAuth 2.0 scopes
//  Authentication requires an email address that matches an actual GMail user (e.g. a machine account)
//  that has appropriate access privileges
func (g *GoogleContacts) initGoogleClient() error {
	googleAuthJson, err := json.Marshal(g.GoogleContactsConfig.GoogleAuth)
	if err != nil {
		return fmt.Errorf("unable to marshal google auth data into json, error: %s", err.Error())
	}

	config, err := google.JWTConfigFromJSON(googleAuthJson, "https://www.google.com/m8/feeds/contacts/")
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %s", err)
	}

	config.Subject = g.GoogleContactsConfig.DelegatedAdminEmail
	g.Client = *config.Client(context.Background())

	return nil
}
