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

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	personnel_sync "github.com/silinternational/personnel-sync"
)

const MaxQuerySize = 10000

const (
	contactFieldID             = "id"
	contactFieldEmail          = "email"
	contactFieldPhoneNumber    = "phoneNumber"
	contactFieldFullName       = "fullName"
	contactFieldGivenName      = "givenName"
	contactFieldFamilyName     = "familyName"
	contactFieldWhere          = "where"
	contactFieldOrganization   = "organization"
	contactFieldTitle          = "title"
	contactFieldJobDescription = "jobDescription"
	contactFieldDepartment     = "department"
)

type GoogleContactsConfig struct {
	DelegatedAdminEmail string
	Domain              string
	GoogleAuth          GoogleAuth
	BatchSize           int
	BatchDelaySeconds   int
}

type GoogleContacts struct {
	DestinationConfig    personnel_sync.DestinationConfig
	GoogleContactsConfig GoogleContactsConfig
	Client               http.Client
}

type Entries struct {
	XMLName xml.Name  `xml:"feed"`
	Entries []Contact `xml:"entry"`
	Total   int       `xml:"totalResults"`
}

type Contact struct {
	XMLName      xml.Name      `xml:"entry"`
	ID           string        `xml:"id"`
	Links        []Link        `xml:"link"`
	Etag         string        `xml:"etag,attr"`
	Title        string        `xml:"title"`
	Name         Name          `xml:"name"`
	Emails       []Email       `xml:"email"`
	PhoneNumbers []PhoneNumber `xml:"phoneNumber"`
	Organization Organization  `xml:"organization"`
	Where        Where         `xml:"where"`
}

type Email struct {
	XMLName xml.Name `xml:"email"`
	Address string   `xml:"address,attr"`
	Primary bool     `xml:"primary,attr"`
}

type PhoneNumber struct {
	XMLName xml.Name `xml:"phoneNumber"`
	Value   string   `xml:",chardata"`
	Primary bool     `xml:"primary,attr"`
}

type Name struct {
	XMLName    xml.Name `xml:"name"`
	FullName   string   `xml:"fullName"`
	GivenName  string   `xml:"givenName"`
	FamilyName string   `xml:"familyName"`
}

type Organization struct {
	XMLName        xml.Name `xml:"organization"`
	Name           string   `xml:"orgName"`
	Title          string   `xml:"orgTitle"`
	JobDescription string   `xml:"orgJobDescription"`
	Department     string   `xml:"orgDepartment"`
}

type Link struct {
	XMLName xml.Name `xml:"link"`
	Rel     string   `xml:"rel,attr"`
	Href    string   `xml:"href,attr"`
}

type Where struct {
	XMLName     xml.Name `xml."where"`
	ValueString string   `xml:"valueString,attr"`
}

func NewGoogleContactsDestination(destinationConfig personnel_sync.DestinationConfig) (personnel_sync.Destination,
	error) {

	if destinationConfig.Type != personnel_sync.DestinationTypeGoogleContacts {
		return nil, fmt.Errorf("invalid config type: %s", destinationConfig.Type)
	}

	var googleContacts GoogleContacts
	// Unmarshal ExtraJSON into GoogleContactsConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &googleContacts.GoogleContactsConfig)
	if err != nil {
		return &GoogleContacts{}, err
	}

	// Defaults
	config := &googleContacts.GoogleContactsConfig
	if config.BatchSize <= 0 {
		config.BatchSize = DefaultBatchSize
	}
	if config.BatchDelaySeconds <= 0 {
		config.BatchDelaySeconds = DefaultBatchDelaySeconds
	}

	googleContacts.DestinationConfig = destinationConfig

	// Initialize Client object
	err = googleContacts.initGoogleClient()
	if err != nil {
		return &GoogleContacts{}, err
	}

	return &googleContacts, nil
}

func (g *GoogleContacts) GetIDField() string {
	return "id"
}

func (g *GoogleContacts) ForSet(syncSetJson json.RawMessage) error {
	// sync sets not implemented for this destination
	return nil
}

func (g *GoogleContacts) ListUsers() ([]personnel_sync.Person, error) {
	href := fmt.Sprintf("https://www.google.com/m8/feeds/contacts/%s/full?max-results=%d",
		g.GoogleContactsConfig.Domain, MaxQuerySize)
	body, err := g.httpRequest(http.MethodGet, href, "", map[string]string{})
	if err != nil {
		return []personnel_sync.Person{}, fmt.Errorf("failed to retrieve user list: %s", err)
	}

	var parsed Entries

	if err := xml.Unmarshal([]byte(body), &parsed); err != nil {
		return []personnel_sync.Person{}, fmt.Errorf("failed to parse xml for user list: %s", err)
	}
	if parsed.Total >= MaxQuerySize {
		return []personnel_sync.Person{}, fmt.Errorf("too many entries in Google Contacts directory")
	}

	return g.extractPersonsFromResponse(parsed.Entries)
}

func (g *GoogleContacts) ApplyChangeSet(
	changes personnel_sync.ChangeSet,
	eventLog chan<- personnel_sync.EventLogItem) personnel_sync.ChangeResults {

	var results personnel_sync.ChangeResults
	var wg sync.WaitGroup

	batchTimer := personnel_sync.NewBatchTimer(g.GoogleContactsConfig.BatchSize,
		g.GoogleContactsConfig.BatchDelaySeconds)

	if g.DestinationConfig.DisableAdd {
		log.Println("Contact creation is disabled.")
	} else {
		for _, toCreate := range changes.Create {
			wg.Add(1)
			go g.addContact(toCreate, &results.Created, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	if g.DestinationConfig.DisableUpdate {
		log.Println("Contact update is disabled.")
	} else {
		for _, toUpdate := range changes.Update {
			wg.Add(1)
			go g.updateContact(toUpdate, &results.Updated, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	if g.DestinationConfig.DisableDelete {
		log.Println("Contact deletion is disabled.")
	} else {
		for _, toUpdate := range changes.Delete {
			wg.Add(1)
			go g.deleteContact(toUpdate, &results.Deleted, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	wg.Wait()

	return results
}

func (g *GoogleContacts) httpRequest(verb string, url string, body string, headers map[string]string) (string,
	error) {

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

func (g *GoogleContacts) extractPersonsFromResponse(contacts []Contact) ([]personnel_sync.Person, error) {
	persons := make([]personnel_sync.Person, len(contacts))
	for i, entry := range contacts {
		id := findSelfLink(entry)
		persons[i] = personnel_sync.Person{
			CompareValue: findPrimaryEmail(entry),
			ID:           id,
			Attributes: map[string]string{
				contactFieldID:             id,
				contactFieldEmail:          findPrimaryEmail(entry),
				contactFieldPhoneNumber:    findPrimaryPhoneNumber(entry),
				contactFieldFullName:       entry.Title,
				contactFieldGivenName:      entry.Name.GivenName,
				contactFieldFamilyName:     entry.Name.FamilyName,
				contactFieldWhere:          entry.Where.ValueString,
				contactFieldOrganization:   entry.Organization.Name,
				contactFieldTitle:          entry.Organization.Title,
				contactFieldJobDescription: entry.Organization.JobDescription,
				contactFieldDepartment:     entry.Organization.Department,
			},
		}
	}

	return persons, nil
}

func findSelfLink(entry Contact) string {
	for _, link := range entry.Links {
		if link.Rel == "self" {
			return link.Href
		}
	}
	return ""
}

func findPrimaryEmail(entry Contact) string {
	for _, email := range entry.Emails {
		if email.Primary {
			return email.Address
		}
	}
	return ""
}

func findPrimaryPhoneNumber(entry Contact) string {
	for _, phone := range entry.PhoneNumbers {
		if phone.Primary {
			return phone.Value
		}
	}
	return ""
}

func (g *GoogleContacts) addContact(
	person personnel_sync.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- personnel_sync.EventLogItem) {

	defer wg.Done()

	href := "https://www.google.com/m8/feeds/contacts/" + g.GoogleContactsConfig.Domain + "/full"
	body := g.createBody(person)
	headers := map[string]string{"Content-Type": "application/atom+xml"}
	if _, err := g.httpRequest(http.MethodPost, href, body, headers); err != nil {
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

// initGoogleClient creates an http Client and adds a JWT config that has the required OAuth 2.0 scopes
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

// createBody inserts attributes into an XML request body. This might be possible using the Go XML library, but
// it would probably take some sort of hack or workaround to get it to insert the "gd:" namespace prefix on the
// tag names.
// WARNING: This updates all fields, even if omitted in the field mapping. A safer implementation would be to
// merge the data retrieved from Google with the data coming from the source.
func (g *GoogleContacts) createBody(person personnel_sync.Person) string {
	const bodyTemplate = `<atom:entry xmlns:atom='http://www.w3.org/2005/Atom' xmlns:gd='http://schemas.google.com/g/2005'>
	<atom:category scheme='http://schemas.google.com/g/2005#kind' term='http://schemas.google.com/contact/2008#contact' />
	<gd:name>
		<gd:fullName>%s</gd:fullName>
		<gd:givenName>%s</gd:givenName>
		<gd:familyName>%s</gd:familyName>
	</gd:name>
	<gd:email rel='http://schemas.google.com/g/2005#work' primary='true' address='%s'/>
	<gd:phoneNumber rel='http://schemas.google.com/g/2005#work' primary='true'>%s</gd:phoneNumber>
	<gd:where valueString='%s'/>
	<gd:organization rel="http://schemas.google.com/g/2005#work" label="Work" primary="true">
		  <gd:orgName>%s</gd:orgName>
		  <gd:orgTitle>%s</gd:orgTitle>
		  <gd:orgJobDescription>%s</gd:orgJobDescription>
		  <gd:orgDepartment>%s</gd:orgDepartment>
	</gd:organization> 
</atom:entry>`

	return fmt.Sprintf(bodyTemplate, person.Attributes[contactFieldFullName], person.Attributes[contactFieldGivenName],
		person.Attributes[contactFieldFamilyName], person.Attributes[contactFieldEmail],
		person.Attributes[contactFieldPhoneNumber], person.Attributes[contactFieldWhere],
		person.Attributes[contactFieldOrganization], person.Attributes[contactFieldTitle],
		person.Attributes[contactFieldJobDescription], person.Attributes[contactFieldDepartment])
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

	body := g.createBody(person)

	_, err = g.httpRequest(http.MethodPut, url, body, map[string]string{
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
	existingContact, err := g.httpRequest(http.MethodGet, url, "", map[string]string{})
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

	_, err = g.httpRequest(http.MethodDelete, url, "", map[string]string{
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
