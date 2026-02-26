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
	StatusCode  int    `json:"status_code"`
	StatusLabel string `json:"status_label,omitempty"`
	Count       int    `json:"count"`
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

type errorLogsRequest struct {
	StartTime string           `json:"start_time,omitempty"`
	EndTime   string           `json:"end_time,omitempty"`
	PageSize  int              `json:"page_size,omitempty"`
	Cursor    string           `json:"cursor,omitempty"`
	Order     string           `json:"order,omitempty"`
	Filter    *errorLogsFilter `json:"filter,omitempty"`
}

type errorLogsFilter struct {
	APIKeys     []string `json:"api_keys,omitempty"`
	FQDNs       []string `json:"fqdns,omitempty"`
	StatusCodes []int    `json:"status_codes,omitempty"`
	RPCMethods  []string `json:"rpc_methods,omitempty"`
}

type errorLogsResponse struct {
	Items      []ErrorLog `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
	HasMore    bool       `json:"has_more"`
}

type errorClassesResponse struct {
	Items []ErrorStats `json:"items"`
}

type errorFacetsResponse struct {
	FQDNs []struct {
		FQDN  string `json:"fqdn"`
		Count int    `json:"count"`
	} `json:"fqdns,omitempty"`
	RPCMethods []struct {
		RPCMethod string `json:"rpc_method"`
		Count     int    `json:"count"`
	} `json:"rpc_methods,omitempty"`
	Origins []struct {
		Origin string `json:"origin"`
		Count  int    `json:"count"`
	} `json:"origins,omitempty"`
	APIKeys []struct {
		APIKey string `json:"api_key"`
		Count  int    `json:"count"`
	} `json:"api_keys,omitempty"`
}

type LogsAPI struct {
	client *Client
}

func NewLogsAPI(client *Client) *LogsAPI {
	return &LogsAPI{client: client}
}

func (l *LogsAPI) Errors(filters map[string]interface{}) ([]ErrorLog, error) {
	req := toErrorLogsRequest(filters)
	if req.PageSize == 0 {
		req.PageSize = 50
	}
	if req.Order == "" {
		req.Order = "desc"
	}

	var payload errorLogsResponse
	err := l.client.Post("/v4/organization/logs/errors", req, &payload)
	return payload.Items, err
}

func (l *LogsAPI) Stats(filters map[string]interface{}) ([]ErrorStats, error) {
	req := toErrorLogsRequest(filters)
	if req.Order == "" {
		req.Order = "desc"
	}

	var payload errorClassesResponse
	err := l.client.Post("/v4/organization/logs/error-classes", req, &payload)
	return payload.Items, err
}

func (l *LogsAPI) Facets(filters map[string]interface{}) (*ErrorFacets, error) {
	req := toErrorLogsRequest(filters)
	if req.Order == "" {
		req.Order = "desc"
	}

	var payload errorFacetsResponse
	err := l.client.Post("/v4/organization/logs/error-facets", req, &payload)
	if err != nil {
		return nil, err
	}

	facets := &ErrorFacets{}
	for _, entry := range payload.FQDNs {
		facets.FQDNs = append(facets.FQDNs, FacetEntry{Value: entry.FQDN, Count: entry.Count})
	}
	for _, entry := range payload.RPCMethods {
		facets.RPCMethods = append(facets.RPCMethods, FacetEntry{Value: entry.RPCMethod, Count: entry.Count})
	}
	for _, entry := range payload.Origins {
		facets.Origins = append(facets.Origins, FacetEntry{Value: entry.Origin, Count: entry.Count})
	}
	for _, entry := range payload.APIKeys {
		facets.APIKeys = append(facets.APIKeys, FacetEntry{Value: entry.APIKey, Count: entry.Count})
	}
	return facets, nil
}

func toErrorLogsRequest(filters map[string]interface{}) errorLogsRequest {
	req := errorLogsRequest{}
	filter := &errorLogsFilter{}

	if value, ok := filters["from"].(string); ok {
		req.StartTime = value
	}
	if value, ok := filters["to"].(string); ok {
		req.EndTime = value
	}
	if value, ok := filters["limit"].(int); ok {
		req.PageSize = value
	}
	if value, ok := filters["cursor"].(string); ok {
		req.Cursor = value
	}

	if value, ok := filters["api_key"].(string); ok && value != "" {
		filter.APIKeys = []string{value}
	}
	if value, ok := filters["fqdn"].(string); ok && value != "" {
		filter.FQDNs = []string{value}
	}
	if value, ok := filters["status_code"].(int); ok && value > 0 {
		filter.StatusCodes = []int{value}
	}
	if value, ok := filters["rpc_method"].(string); ok && value != "" {
		filter.RPCMethods = []string{value}
	}

	if len(filter.APIKeys) > 0 || len(filter.FQDNs) > 0 || len(filter.StatusCodes) > 0 || len(filter.RPCMethods) > 0 {
		req.Filter = filter
	}
	return req
}
