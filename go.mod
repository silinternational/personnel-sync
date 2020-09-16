module github.com/silinternational/personnel-sync/v4

go 1.14

replace github.com/silinternational/personnel-sync/v4 => ./

require (
	github.com/Jeffail/gabs/v2 v2.5.1
	golang.org/x/net v0.0.0-20200822124328-c89045814202
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	golang.org/x/tools v0.0.0-20200916140129-56d9a0cd3487 // indirect
	google.golang.org/api v0.32.0
)
