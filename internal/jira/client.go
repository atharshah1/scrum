package jira

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"

    "github.com/atharshah1/scrum/internal/auth"
)

type Issue struct {
    Key    string `json:"key"`
    Fields struct {
        Summary string `json:"summary"`
        Status  struct {
            Name string `json:"name"`
        } `json:"status"`
    } `json:"fields"`
}

type User struct {
    AccountID    string `json:"accountId"`
    DisplayName  string `json:"displayName"`
    EmailAddress string `json:"emailAddress"`
}

type Project struct {
    Key  string `json:"key"`
    Name string `json:"name"`
}

type SearchResult struct {
    Issues []Issue `json:"issues"`
}

type SearchRequest struct {
    JQL    string   `json:"jql"`
    Fields []string `json:"fields,omitempty"`
}

type CreateRequest struct {
    Fields struct {
        Project struct {
            Key string `json:"key"`
        } `json:"project"`
        Summary   string `json:"summary"`
        IssueType struct {
            Name string `json:"name"`
        } `json:"issuetype"`
        Assignee struct {
            AccountID string `json:"accountId,omitempty"`
        } `json:"assignee,omitempty"`
    } `json:"fields"`
}

type CreateResponse struct {
    Key string `json:"key"`
    ID  string `json:"id"`
}

type CreateProjectRequest struct {
    Key                string `json:"key"`
    Name               string `json:"name"`
    ProjectTypeKey     string `json:"projectTypeKey"`
    ProjectTemplateKey string `json:"projectTemplateKey"`
    AssigneeType       string `json:"assigneeType"`
    LeadAccountId      string `json:"leadAccountId"`
}

type CreateProjectResponse struct {
    Key string `json:"key"`
    ID  int    `json:"id"`
}

type Transition struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    To   struct {
        Name string `json:"name"`
    } `json:"to"`
}

type TransitionRequest struct {
    Transition struct {
        ID string `json:"id"`
    } `json:"transition"`
}

type Comment struct {
    ID      string                 `json:"id"`
    Author  struct {
        DisplayName string `json:"displayName"`
    } `json:"author"`
    Body    map[string]interface{} `json:"body"`
    Created string                 `json:"created"`
}

type Client struct{}

func (c *Client) SearchIssues(jql string) ([]Issue, error) {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return nil, err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return nil, err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/search/jql", cloudID)

    reqBody := SearchRequest{
        JQL:    jql,
        Fields: []string{"summary", "status"},
    }
    b, _ := json.Marshal(reqBody)

    req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(b))
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("search failed (%s): %s", resp.Status, string(b))
    }

    var result SearchResult
    err = json.NewDecoder(resp.Body).Decode(&result)
    return result.Issues, err
}

func (c *Client) CreateIssue(projectKey, summary, issueType, assigneeID string) (string, error) {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return "", err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return "", err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/issue", cloudID)

    reqBody := CreateRequest{}
    reqBody.Fields.Project.Key = projectKey
    reqBody.Fields.Summary = summary
    reqBody.Fields.IssueType.Name = issueType
    if assigneeID != "" {
        reqBody.Fields.Assignee.AccountID = assigneeID
    }

    b, _ := json.Marshal(reqBody)

    req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(b))
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        b, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("failed to create issue (%s): %s", resp.Status, string(b))
    }

    var result CreateResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }

    return result.Key, nil
}

func (c *Client) GetTransitions(issueKey string) ([]Transition, error) {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return nil, err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return nil, err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/issue/%s/transitions", cloudID, issueKey)

    req, _ := http.NewRequest("GET", apiURL, nil)
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to get transitions (%s): %s", resp.Status, string(b))
    }

    var result struct {
        Transitions []Transition `json:"transitions"`
    }
    err = json.NewDecoder(resp.Body).Decode(&result)
    return result.Transitions, err
}

func (c *Client) TransitionIssue(issueKey, transitionID string) error {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/issue/%s/transitions", cloudID, issueKey)

    reqBody := TransitionRequest{}
    reqBody.Transition.ID = transitionID
    b, _ := json.Marshal(reqBody)

    req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(b))
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusNoContent {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to transition issue (%s): %s", resp.Status, string(b))
    }

    return nil
}

func (c *Client) AssignIssue(issueKey, accountID string) error {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/issue/%s/assignee", cloudID, issueKey)

    reqBody := map[string]string{"accountId": accountID}
    b, _ := json.Marshal(reqBody)

    req, _ := http.NewRequest("PUT", apiURL, bytes.NewBuffer(b))
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusNoContent {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to assign issue (%s): %s", resp.Status, string(b))
    }

    return nil
}

func (c *Client) GetComments(issueKey string) ([]Comment, error) {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return nil, err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return nil, err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/issue/%s/comment", cloudID, issueKey)

    req, _ := http.NewRequest("GET", apiURL, nil)
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to get comments (%s): %s", resp.Status, string(b))
    }

    var result struct {
        Comments []Comment `json:"comments"`
    }
    err = json.NewDecoder(resp.Body).Decode(&result)
    return result.Comments, err
}

func (c *Client) AddComment(issueKey, body string) error {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/issue/%s/comment", cloudID, issueKey)

    // Parse body for mentions @[User Query]
    var content []interface{}

    parts := strings.Split(body, "@[")
    for i, part := range parts {
        if i == 0 {
            if part != "" {
                content = append(content, map[string]interface{}{"type": "text", "text": part})
            }
            continue
        }

        // part is like "Name] rest of text"
        subParts := strings.SplitN(part, "]", 2)
        if len(subParts) == 2 {
            query := subParts[0]
            rest := subParts[1]

            // Search user
            users, _ := c.FindUsers(query) // Ignore error, treat as text if fail
            if len(users) > 0 {
                user := users[0]
                content = append(content, map[string]interface{}{
                    "type": "mention",
                    "attrs": map[string]interface{}{
                        "id":   user.AccountID,
                        "text": "@" + user.DisplayName,
                    },
                })
            } else {
                // Fallback to text if user not found
                content = append(content, map[string]interface{}{"type": "text", "text": "@[" + query + "]"})
            }

            if rest != "" {
                content = append(content, map[string]interface{}{"type": "text", "text": rest})
            }
        } else {
            // No closing bracket, treat as text
            content = append(content, map[string]interface{}{"type": "text", "text": "@[" + part})
        }
    }

    // Construct ADF (Atlassian Document Format)
    reqBody := map[string]interface{}{
        "body": map[string]interface{}{
            "type":    "doc",
            "version": 1,
            "content": []interface{}{
                map[string]interface{}{
                    "type": "paragraph",
                    "content": content,
                },
            },
        },
    }

    b, _ := json.Marshal(reqBody)

    req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(b))
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to add comment (%s): %s", resp.Status, string(b))
    }

    return nil
}

func (c *Client) GetProjects() ([]Project, error) {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return nil, err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return nil, err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/project", cloudID)

    req, _ := http.NewRequest("GET", apiURL, nil)
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to get projects (%s): %s", resp.Status, string(b))
    }

    var projects []Project
    err = json.NewDecoder(resp.Body).Decode(&projects)
    return projects, err
}

func (c *Client) GetMyself() (*User, error) {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return nil, err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return nil, err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/myself", cloudID)

    req, _ := http.NewRequest("GET", apiURL, nil)
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var user User
    err = json.NewDecoder(resp.Body).Decode(&user)
    return &user, err
}

func (c *Client) FindUsers(query string) ([]User, error) {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return nil, err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return nil, err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/user/search?query=%s", cloudID, url.QueryEscape(query))

    req, _ := http.NewRequest("GET", apiURL, nil)
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var users []User
    err = json.NewDecoder(resp.Body).Decode(&users)
    return users, err
}

func (c *Client) CreateProject(key, name, leadAccountId string) (string, error) {
    token, err := auth.RefreshAccessToken()
    if err != nil {
        return "", err
    }

    cloudID, err := auth.GetCloudID()
    if err != nil {
        return "", err
    }

    apiURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3/project", cloudID)

    reqBody := CreateProjectRequest{
        Key:                key,
        Name:               name,
        ProjectTypeKey:     "software",
        ProjectTemplateKey: "com.pyxis.greenhopper.jira:gh-simplified-scrum-classic",
        AssigneeType:       "UNASSIGNED",
        LeadAccountId:      leadAccountId,
    }

    b, _ := json.Marshal(reqBody)

    req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(b))
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        b, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("failed to create project (%s): %s", resp.Status, string(b))
    }

    var result CreateProjectResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }

    return result.Key, nil
}