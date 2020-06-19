module github.com/silinternational/personnel-sync/v3

go 1.14

replace github.com/silinternational/personnel-sync/v3 => ./

require (
	github.com/Jeffail/gabs v1.4.0
	github.com/Jeffail/gabs/v2 v2.5.1
	github.com/aws/aws-lambda-go v1.16.0
	golang.org/x/net v0.0.0-20200519113804-d87ec0cfa476
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/api v0.24.0
)
