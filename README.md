# Personnel Sync
This application is intended to provide a fairly easy to use way of synchronizing people from a personnel system 
to some other system. In this application there are data _sources_ and _destinations_. Since _destinations_ have their 
own unique APIs and integration methods each _destination_ is developed individually to implement the _Destination_ 
interface. The runtime for this application is configured using a `config.json` file. An example is provided named 
`config.example.json`, however it only has the `GoogleGroups` destination in it so other supported destinations are 
documented below. 

## Sources

### REST API
Data sources coming from simple API calls can use the `RestAPI` source. Here are some examples of how to configure it:

#### Basic Authentication
```json
{
  "Source": {
    "Type": "RestAPI",
    "ExtraJSON": {
      "Method": "GET",
      "BaseURL": "https://example.com",
      "Path": "/path",
      "ResultsJSONContainer": "Results",
      "AuthType": "basic",
      "Username": "username",
      "Password": "password",
      "CompareAttribute": "email"
    }
  }
}
```

#### Bearer Token Authentication
```json
{
  "Source": {
    "Type": "RestAPI",
    "ExtraJSON": {
      "Method": "GET",
      "BaseURL": "https://example.com",
      "Path": "/path",
      "ResultsJSONContainer": "Results",
      "AuthType": "bearer",
      "Password": "token",
      "CompareAttribute": "email"
    }
  }
}
```

#### Salesforce OAuth Authentication
```json
{
  "Source": {
    "Type": "RestAPI",
    "ExtraJSON": {
      "Method": "GET",
      "BaseURL": "https://login.salesforce.com/services/oauth2/token",
      "Path": "/services/data/v20.0/query/",
      "ResultsJSONContainer": "records",
      "AuthType": "SalesforceOauth",
      "Username": "admin@example.com",
      "Password": "LqznAW6N8.EenJVT",
      "ClientID": "VczVNcM8xaDRB8bi_fLyn2BJzpG6bihUxNQGeV2BePM4FBT2VMeJfGnC38K46aqBRLTCJy.GJK2RmPUCVrm39",
      "ClientSecret": "2CD6093EFA0DABCFABE3B7B78F951EFD1B59283E23D357EB458AE6852838C26C",
      "CompareAttribute": "email"
    }
  }
}
```

## Destinations

### Google Groups
This destination is useful for keeping Google Groups in sync with reports from a personnel system. Below is an example 
of the destination configuration required for Google Groups:

```json
{
  "Destination": {
    "Type": "GoogleGroups",
    "ExtraJSON": {
      "BatchSizePerMinute": 50,
      "DelegatedAdminEmail": "delegated-admin@domain.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }
    }
  },
  "AttributeMap": [
    {
      "Source": "First_Name",
      "Destination": "givenName",
      "required": true
    },
    {
      "Source": "Last_Name",
      "Destination": "sn",
      "required": true
    },
    {
      "Source": "Email",
      "Destination": "mail",
      "required": true
    }
  ],
  "SyncSets": [
    {
      "Name": "Sync from personnel to Google Groups",
      "Source": {
          "Path": "/user-report"
      },
      "Destination": {
          "GroupEmail": "group1@groups.domain.com",
          "Owners": ["person_a@domain.com","person_b@domain.com"],
          "Managers": ["another_person@domain.com", "yet-another-person@domain.com"],
          "ExtraOwners": ["google-admin@domain.com"]
      }
    }
  ]
}
```

### Google Users
This destination can update User records in the Google Directory. Presently, only the user's name is available
for updating, but other fields may be added in the future. Following is an example configuration:

```json
  "Destination": {
    "Type": "GoogleUsers",
    "ExtraJSON": {
      "BatchSizePerMinute": 50,
      "DelegatedAdminEmail": "admin@example.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }            
    }
  },
  "AttributeMap": [
    {
      "Source": "email",
      "Destination": "email",
      "required": true
    },
    {
      "Source": "last_name",
      "Destination": "familyName",
      "required": true
    },
    {
      "Source": "first_name",
      "Destination": "givenName",
      "required": true
    }
  ],
```

#### Google Service Account Configuration

(see https://stackoverflow.com/questions/53808710/authenticate-to-google-admin-directory-api#answer-53808774 and
 https://developers.google.com/admin-sdk/reports/v1/guides/delegation)

In the [Google Developer Console](https://console.developers.google.com) ...
* Enable the appropriate API for the Service Account.
* Create a new Service Account and a corresponding JSON credential file, which should contain something like this:

```json
  {
    "type": "service_account",
    "project_id": "abc-theme-123456",
    "private_key_id": "abc123",
    "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
    "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
    "client_id": "123456789012345678901",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
    "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
  }
```

These contents will need to be copied into the `config.json` file as the value of the `GoogleAuth` key under 
`Destination`/`ExtraJSON`.

In [Google Admin Security](https://admin.google.com/AdminHome?hl=en#SecuritySettings:) ...
* Under "Advanced Settings" add the appropriate API Scopes to the Service Account. Use the numeric `client_id`.
* API Scopes required for Google Groups are: `https://www.googleapis.com/auth/admin.directory.group` and 
`https://www.googleapis.com/auth/admin.directory.group.member`
* The API Scope required for Google User Directory is: `https://www.googleapis.com/auth/admin.directory.user`

The sync job will need to use the Service Account credentials to impersonate another user that has
appropriate domain privileges and who has logged in at least once into G Suite and
accepted the terms and conditions. The email address for this user should be stored in the `config.json`
as the `DelegatedAdminEmail` value under `Destination`/`ExtraJSON`.

## SolarWinds WebHelpDesk


```json
{
  "AttributeMap": [
      {
        "Source": "FIRST_NAME",
        "Destination": "firstName",
        "required": true,
        "CaseSensitive": true
      },
      {
        "Source": "LAST_NAME",
        "Destination": "lastName",
        "required": true,
        "CaseSensitive": true
      },
      {
        "Source": "EMAIL",
        "Destination": "email",
        "required": true,
        "CaseSensitive": false
      },
      {
        "Source": "USER_NAME",
        "Destination": "username",
        "required": true,
        "CaseSensitive": false
      },
      {
        "Source": "Staff_ID",
        "Destination": "employmentStatus"
      }
  ],
  "Destination": {
    "Type": "WebHelpDesk",
    "ExtraJSON": {
      "URL": "https://whd.mycompany.com/helpdesk/WebObjects/Helpdesk.woa",
      "Username": "syncuser",
      "Password": "apitoken",
      "ListClientsPageLimit": 100,
      "BatchSizePerMinute": 50
    }
  }
}
```

`ListClientsPageLimit` and `BatchSizePerMinute` are optional. Their defaults are as shown in the example config.
