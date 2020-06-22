package restapi

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
)

type fakeEndpoint struct {
	path             string
	method           string
	status           int
	responseBody     string
	authType         string
	username         string
	password         string
	compareAttr      string
	resultsContainer string
}

const (
	EndpointListWorkday    = "list workday"
	EndpointListOther      = "list other"
	EndpointListSalesforce = "list salesforce"
	EndpointCreateOther    = "create other"
)

const extraJSONtemplate = `{
  "Method": "%s",
  "BaseURL": "%s",
  "ResultsJSONContainer": "%s",
  "AuthType": "%s",
  "Username": "%s",
  "Password": "%s",
  "CompareAttribute": "%s"
}`

const workdayUsersJSON = `{
  "Report_Entry": [
    {
      "Employee_Number": "10013",
      "First_Name": "Mickey",
      "Last_Name": "Mouse",
      "Display_Name": "Mickey Mouse",
      "Username": "MICKEY_MOUSE",
      "Email": "mickey_mouse@acme.com",
      "Personal_Email": "mickey_mouse@mousemail.com",
      "Account_Locked__Disabled_or_Expired": "0",
      "requireMfa": "0",
      "Company": "Disney"
    },
	{
      "Employee_Number": "10011",
      "First_Name": "Donald",
      "Last_Name": "Duck",
      "Display_Name": "Donald Duck",
      "Username": "DONALD_DUCK",
      "Email": "donald_duck@acme.com",
      "Personal_Email": "donald_duck@duckmail.com",
      "Account_Locked__Disabled_or_Expired": "0",
      "requireMfa": "0",
      "Company": "Disney"
    }
  ]
}`

const otherUsersJSON = `[
    {
      "employeeID": "10013",
      "first": "Mickey",
      "last": "Mouse",
      "display": "Mickey Mouse",
      "username": "MICKEY_MOUSE",
      "email": "mickey_mouse@acme.com"
    },
	{
      "employeeID": "10011",
      "first": "Donald",
      "last": "Duck",
      "display": "Donald Duck",
      "username": "DONALD_DUCK",
      "email": "donald_duck@acme.com"
    }
]`

const salesforceUsersJSON = `{
  "totalSize": 2,
  "done": true,
  "records": [
    {
      "attributes": {
        "type": "fHCM2__Team_Member__c",
        "url": "/services/data/v20.0/sobjects/fHCM2__Team_Member__c/a1H1U737901ULOwUAO"
      },
      "Name": "Mickey Mouse",
      "fHCM2__User__r": {
        "attributes": {
          "type": "User",
          "url": "/services/data/v20.0/sobjects/User/0051U579303drCrQAI"
        },
        "Email": "mickey_mouse@acme.com"
      }
    },
    {
      "attributes": {
        "type": "fHCM2__Team_Member__c",
        "url": "/services/data/v20.0/sobjects/fHCM2__Team_Member__c/a1H1U50361ULZbUAO"
      },
      "Name": "Donald Duck",
      "fHCM2__User__r": {
        "attributes": {
          "type": "User",
          "url": "/services/data/v20.0/sobjects/User/0051U773763dqt3QAA"
        },
        "Email": "donald_duck@acme.com"
      }
    }
  ]
}`

func getFakeEndpoints() map[string]fakeEndpoint {
	return map[string]fakeEndpoint{
		EndpointListWorkday: {
			path:             "/workday",
			method:           http.MethodGet,
			status:           http.StatusOK,
			responseBody:     workdayUsersJSON,
			authType:         AuthTypeBasic,
			username:         "workday_username",
			password:         "workday_password",
			compareAttr:      "Email",
			resultsContainer: "Report_Entry",
		},
		EndpointListOther: {
			path:             "/other/list",
			method:           http.MethodGet,
			status:           http.StatusOK,
			responseBody:     otherUsersJSON,
			authType:         AuthTypeBearer,
			password:         "bearer_token",
			compareAttr:      "email",
			resultsContainer: "",
		},
		EndpointListSalesforce: {
			path:             "/sfdc",
			method:           http.MethodGet,
			status:           http.StatusOK,
			responseBody:     salesforceUsersJSON,
			authType:         AuthTypeSalesforceOauth,
			username:         "sf_username",
			password:         "sf_token",
			compareAttr:      "fHCM2__User__r.Email",
			resultsContainer: "records",
		},
		EndpointCreateOther: {
			path:             "/other/create",
			method:           http.MethodPost,
			status:           http.StatusOK,
			authType:         AuthTypeBearer,
			password:         "bearer_token",
			compareAttr:      "",
			resultsContainer: "",
		},
	}
}

func getTestServer() *httptest.Server {
	mux := http.NewServeMux()
	endpoints := getFakeEndpoints()
	for name := range endpoints {
		e := endpoints[name]
		status := e.status
		responseBody := e.responseBody
		mux.HandleFunc(e.path, func(w http.ResponseWriter, req *http.Request) {
			switch e.authType {
			case AuthTypeBasic:
				user, pass, ok := req.BasicAuth()
				if !ok || e.username != user || e.password != pass {
					status = http.StatusUnauthorized
					responseBody = `{"error": "Not Authorized"}`
				}
			case AuthTypeBearer, AuthTypeSalesforceOauth:
				token := req.Header.Get("Authorization")
				if "Bearer "+e.password != token {
					status = http.StatusUnauthorized
					responseBody = `{"error": "Not Authorized"}`
				}
			}

			// basic check to see if a POST has a request body
			bodyBytes, err := ioutil.ReadAll(req.Body)
			if err != nil {
				status = http.StatusBadRequest
			}
			bodyString := string(bodyBytes)
			if req.Method == http.MethodPost && bodyString == "" {
				status = http.StatusBadRequest
				responseBody = `{"error":"empty request body"}`
			}

			w.WriteHeader(status)
			w.Header().Set("content-type", "application/json")
			_, _ = io.WriteString(w, responseBody)
		})
	}
	return httptest.NewServer(mux)
}
