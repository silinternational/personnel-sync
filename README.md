# Personnel Sync
This application is intended to provide a fairly easy to use way of synchronizing people from a personnel system 
to some other system. In this application there are data _sources_ and _destinations_. Since _destinations_ have their 
own unique APIs and integration methods each _destination_ is developed individually to implement the _Destination_ 
interface. The runtime for this application is configured using a `config.json` file. An example is provided named 
`config.example.json`, however it only has the `GoogleGroups` destination in it so other supported destinations are 
documented below. 

## Destinations

### Google Groups
This destination is useful for keeping Google Groups in sync with reports from a personnel system. Below is an example 
of the destination configuration required for Google Groups:

```json
{
  "Type": "GoogleGroups",
  "URL": "notused",
  "Username": "notused",
  "Password": "notused",
  "ExtraJSON": {
    "DelegatedAdminEmail": "delegated-admin@domain.com",
    "GroupEmail": "group1@groups.domain.com",
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
}
```

#### Google Service Account Configuration

(see https://stackoverflow.com/questions/53808710/authenticate-to-google-admin-directory-api#answer-53808774 and
 https://developers.google.com/admin-sdk/reports/v1/guides/delegation)

In the google developer console ...
* Create a new Service Account and a corresponding JSON credential file.
* Delegate Domain-Wide Authority to the Service Account.
* The email address for this user should be stored in the `config.json` as the `GoogleDelegatedAdmin` value

The JSON credential file should contain something like this ...

```json
{
  "DestinationAttributeMap": [
    {
      "SourceName": "Email",
      "DestinationName": "email",
      "required": true
    }
  ],
  "Destination": {
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
```

These contents will need to be copied into the `config.json` file as the value of the `GoogleAuth` key under 
`Destination`/`ExtraJSON`.

The sync job will need to use the Service Account credentials to impersonate another user that has
domain superadmin privilege and who has logged in at least once into G Suite and
accepted the terms and conditions.

## SolarWinds WebHelpDesk


```json
{
  "DestinationAttributeMap": [
      {
        "SourceName": "FIRST_NAME",
        "DestinationName": "firstName",
        "required": true
      },
      {
        "SourceName": "LAST_NAME",
        "DestinationName": "lastName",
        "required": true
      },
      {
        "SourceName": "EMAIL",
        "DestinationName": "email",
        "required": true
      },
      {
        "SourceName": "USER_NAME",
        "DestinationName": "username",
        "required": true
      },
      {
        "SourceName": "Staff_ID",
        "DestinationName": "employmentStatus"
      }
  ],
  "Destination": {
    "Type": "WebHelpDesk",
    "URL": "https://whd.mycompany.com/helpdesk/WebObjects/Helpdesk.woa",
    "Username": "syncuser",
    "Password": "apitoken",
    "ExtraJSON": {
      "AccountID": "do we need this for a hosted install?",
      "ListClientsPageLimit": 100
    }
  }
}
```