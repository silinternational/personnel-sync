package googledest

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	personnel_sync "github.com/silinternational/personnel-sync"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
)

const MaxQuerySize = 10000

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
	Total   int       `xml:"totalResults"`
}

type Contact struct {
	XMLName xml.Name `xml:"entry"`
	ID      string   `xml:"id"`
	Links   []Link   `xml:"link"`
	Etag    string   `xml:"etag,attr"`
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

type Link struct {
	XMLName xml.Name `xml:"link"`
	Rel     string   `xml:"rel,attr"`
	Href    string   `xml:"href,attr"`
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

func (g *GoogleContacts) httpRequest(verb string, url string, body string, headers map[string]string) (string, error) {
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

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("GData-Version", "3.0")
	req.Header.Set("User-Agent", "personnel-sync")

	resp, err := g.Client.Do(req)
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

func (g *GoogleContacts) ListUsers() ([]personnel_sync.Person, error) {
	href := "https://www.google.com/m8/feeds/contacts/" + g.GoogleContactsConfig.Domain + "/full?max-results=" + strconv.Itoa(MaxQuerySize)
	body, err := g.httpRequest("GET", href, "", map[string]string{})
	if err != nil {
		return []personnel_sync.Person{}, fmt.Errorf("failed to retrieve user list: %s", err)
	}

	var parsed Entries

	err = xml.Unmarshal([]byte(body), &parsed)
	if err != nil {
		return []personnel_sync.Person{}, fmt.Errorf("failed to parse xml for user list: %s", err)
	}
	if parsed.Total >= MaxQuerySize {
		return []personnel_sync.Person{}, fmt.Errorf("Google Contacts directory contains too many entries")
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

		var selfLink string
		for j, l := range parsed.Entries[i].Links {
			if l.Rel == "self" {
				selfLink = parsed.Entries[i].Links[j].Href
				break
			}
		}

		persons[i] = personnel_sync.Person{
			CompareValue: primaryEmail,
			ID:           selfLink,
			Attributes: map[string]string{
				"email":    primaryEmail,
				"fullName": parsed.Entries[i].Title,
			},
		}
	}

	return persons, nil
}

func (g *GoogleContacts) ApplyChangeSet(
	changes personnel_sync.ChangeSet,
	eventLog chan<- personnel_sync.EventLogItem) personnel_sync.ChangeResults {

	var results personnel_sync.ChangeResults
	var wg sync.WaitGroup

	// One minute per batch
	batchTimer := personnel_sync.NewBatchTimer(g.BatchSizePerMinute, int(60))

	for _, toCreate := range changes.Create {
		wg.Add(1)
		go g.addContact(toCreate, &results.Created, &wg, eventLog)
		batchTimer.WaitOnBatch()
	}

	for _, toUpdate := range changes.Update {
		wg.Add(1)
		go g.updateContact(toUpdate, &results.Updated, &wg, eventLog)
		batchTimer.WaitOnBatch()
	}

	for _, toUpdate := range changes.Delete {
		wg.Add(1)
		go g.deleteContact(toUpdate, &results.Deleted, &wg, eventLog)
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

	body := g.createBody(person)

	_, err := g.httpRequest("POST", href, body, map[string]string{"Content-Type": "application/atom+xml"})
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("unable to insert %s in Google contacts: %s", person.CompareValue, err)}
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
		return fmt.Errorf("unable to marshal google auth data into json, error: %s", err)
	}

	config, err := google.JWTConfigFromJSON(googleAuthJson, "https://www.google.com/m8/feeds/contacts/")
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %s", err)
	}

	config.Subject = g.GoogleContactsConfig.DelegatedAdminEmail
	g.Client = *config.Client(context.Background())

	return nil
}

func (g *GoogleContacts) createBody(person personnel_sync.Person) string {
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
	return fmt.Sprintf(bodyTemplate, person.Attributes["fullName"], person.Attributes["email"])
}

func (g *GoogleContacts) updateContact(
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	url := person.ID

	contact, err := g.getContact(url)
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("failed retrieving contact %s: %s", person.CompareValue, err)}
		return
	}

	// Update all fields with data from the source -- note that this is a bit dangerous because any
	// fields not included will be erased in Google. A safer solution would be to merge the data
	// retrieved from Google with the data coming from the source.
	body := g.createBody(person)

	_, err = g.httpRequest("PUT", url, body, map[string]string{
		"If-Match":     contact.Etag,
		"Content-Type": "application/atom+xml",
	})
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("updateUser failed updating user %s: %s", person.CompareValue, err)}
		return
	}

	atomic.AddUint64(counter, 1)
}

func (g *GoogleContacts) getContact(url string) (Contact, error) {
	existingContact, err := g.httpRequest("GET", url, "", map[string]string{})
	if err != nil {
		return Contact{}, fmt.Errorf("GET failed: %s", err)
	}

	var c Contact
	err = xml.Unmarshal([]byte(existingContact), &c)
	if err != nil {
		return Contact{}, fmt.Errorf("failed to parse xml: %s", err)
	}

	return c, nil
}

func (g *GoogleContacts) deleteContact(
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	url := person.ID

	contact, err := g.getContact(url)
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("failed retrieving contact %s: %s", person.CompareValue, err)}
		return
	}

	_, err = g.httpRequest("DELETE", url, "", map[string]string{
		"If-Match": contact.Etag,
	})
	if err != nil {
		eventLog <- personnel_sync.EventLogItem{
			Event:   "error",
			Message: fmt.Sprintf("deleteUser failed deleting user %s: %s", person.CompareValue, err)}
		return
	}

	atomic.AddUint64(counter, 1)
}
