package api

type ErrorLog struct {
	Timestamp        string `json:"timestamp"`
	RequestID        string `json:"request_id"`
	APIKey           string `json:"api_key"`
	FQDN             string `json:"fqdn"`
	StatusCode       int    `json:"response_status_code"`
	StatusLabel      string `json:"response_status_label"`
	RPCMethods       string `json:"request_rpc_methods"`
	HTTPMethod       string `json:"request_http_method"`
	ErrorMessage     string `json:"error_message"`
	BackendLatencyMs int    `json:"backend_latency_ms"`
	TotalLatencyMs   int    `json:"total_latency_ms"`
}

type ErrorStats struct {
	StatusCode int `json:"status_code"`
	Count      int `json:"count"`
}

type ErrorFacets struct {
	FQDNs      []FacetEntry `json:"fqdns,omitempty"`
	RPCMethods []FacetEntry `json:"rpc_methods,omitempty"`
	Origins    []FacetEntry `json:"origins,omitempty"`
	APIKeys    []FacetEntry `json:"api_keys,omitempty"`
}

type FacetEntry struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type LogsAPI struct {
	client *Client
}

func NewLogsAPI(client *Client) *LogsAPI {
	return &LogsAPI{client: client}
}

func (l *LogsAPI) Errors(filters map[string]interface{}) ([]ErrorLog, error) {
	var logs []ErrorLog
	err := l.client.Post("/v4/organization/logs/errors", filters, &logs)
	return logs, err
}

func (l *LogsAPI) Stats(filters map[string]interface{}) ([]ErrorStats, error) {
	var stats []ErrorStats
	err := l.client.Post("/v4/organization/logs/errors/status_summary", filters, &stats)
	return stats, err
}

func (l *LogsAPI) Facets(filters map[string]interface{}) (*ErrorFacets, error) {
	var facets ErrorFacets
	err := l.client.Post("/v4/organization/logs/errors/facets", filters, &facets)
	return &facets, err
}
