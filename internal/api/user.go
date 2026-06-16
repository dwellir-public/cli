package api

type UserInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

type UserAPI struct {
	client *Client
}

func NewUserAPI(client *Client) *UserAPI {
	return &UserAPI{client: client}
}

func (u *UserAPI) Current() (*UserInfo, error) {
	var user UserInfo
	err := u.client.Get("/v4/user", nil, &user)
	return &user, err
}
