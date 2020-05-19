package googledest

const DefaultBatchSize = 10
const DefaultBatchDelaySeconds = 3

type GoogleConfig struct {
	DelegatedAdminEmail string
	Domain              string
	GoogleAuth          GoogleAuth
	BatchSize           int
	BatchDelaySeconds   int
}
