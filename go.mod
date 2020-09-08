module github.com/silinternational/personnel-sync/v4

go 1.14

replace github.com/silinternational/personnel-sync/v4 => ./

require (
	github.com/Jeffail/gabs/v2 v2.5.1
	golang.org/x/net v0.0.0-20200822124328-c89045814202
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/api <v0.31.0
)
