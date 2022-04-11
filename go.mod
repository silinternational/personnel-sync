module github.com/silinternational/personnel-sync/v6

go 1.14

replace github.com/silinternational/personnel-sync/v6 => ./

require (
	github.com/Jeffail/gabs/v2 v2.5.1
	github.com/aws/aws-lambda-go v1.19.1
	github.com/aws/aws-sdk-go v1.34.33
	github.com/stretchr/testify v1.6.1
	golang.org/x/net v0.0.0-20200822124328-c89045814202
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	google.golang.org/api v0.32.0
)
