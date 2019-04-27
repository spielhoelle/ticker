package api

import (
	"net/http"
	"strconv"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/gin-gonic/gin"

	"github.com/pkg/errors"
	"github.com/systemli/ticker/internal/bridge"
	. "github.com/systemli/ticker/internal/model"
	. "github.com/systemli/ticker/internal/storage"
	"github.com/systemli/ticker/internal/util"
)

//GetTickersHandler returns all Ticker with paging
func GetTickersHandler(c *gin.Context) {
	me, err := Me(c)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, ErrorUserNotFound))
		return
	}

	var tickers []*Ticker
	if me.IsSuperAdmin {
		err = DB.All(&tickers, storm.Reverse())
	} else {
		allowed := me.Tickers
		err = DB.Select(q.In("ID", allowed)).Reverse().Find(&tickers)
		if err == storm.ErrNotFound {
			err = nil
			tickers = []*Ticker{}
		}
	}
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	c.JSON(http.StatusOK, NewJSONSuccessResponse("tickers", NewTickersResponse(tickers)))
}

//GetTickerHandler returns a Ticker for the given id
func GetTickerHandler(c *gin.Context) {
	me, err := Me(c)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, ErrorUserNotFound))
		return
	}

	tickerID, err := strconv.Atoi(c.Param("tickerID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	if !me.IsSuperAdmin {
		if !contains(me.Tickers, tickerID) {
			c.JSON(http.StatusForbidden, NewJSONErrorResponse(ErrorCodeInsufficientPermissions, ErrorInsufficientPermissions))
			return
		}
	}

	var ticker Ticker
	err = DB.One("ID", tickerID, &ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, NewJSONSuccessResponse("ticker", NewTickerResponse(&ticker)))
}

//GetTickerUsersHandler returns Users for the given ticker
func GetTickerUsersHandler(c *gin.Context) {
	me, err := Me(c)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, ErrorUserNotFound))
		return
	}

	tickerID, err := strconv.Atoi(c.Param("tickerID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	var ticker Ticker
	err = DB.One("ID", tickerID, &ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeNotFound, err.Error()))
		return
	}

	if !me.IsSuperAdmin {
		if !contains(me.Tickers, tickerID) {
			c.JSON(http.StatusForbidden, NewJSONErrorResponse(ErrorCodeInsufficientPermissions, ErrorInsufficientPermissions))
			return
		}
	}

	//TODO: Discuss need of Pagination
	users, _ := FindUsersByTicker(ticker)

	c.JSON(http.StatusOK, NewJSONSuccessResponse("users", NewUsersResponse(users)))
}

//PostTickerHandler creates and returns a new Ticker
func PostTickerHandler(c *gin.Context) {
	if !IsAdmin(c) {
		c.JSON(http.StatusForbidden, NewJSONErrorResponse(ErrorCodeInsufficientPermissions, ErrorInsufficientPermissions))
		return
	}

	ticker := NewTicker()
	err := updateTicker(ticker, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	err = DB.Save(ticker)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	c.JSON(http.StatusOK, NewJSONSuccessResponse("ticker", NewTickerResponse(ticker)))
}

//PutTickerHandler updates and returns a existing Ticker
func PutTickerHandler(c *gin.Context) {
	me, err := Me(c)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, ErrorUserNotFound))
		return
	}

	tickerID, err := strconv.Atoi(c.Param("tickerID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	if !me.IsSuperAdmin {
		if !contains(me.Tickers, tickerID) {
			c.JSON(http.StatusForbidden, NewJSONErrorResponse(ErrorCodeInsufficientPermissions, ErrorInsufficientPermissions))
			return
		}
	}

	var ticker Ticker
	err = DB.One("ID", tickerID, &ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	err = updateTicker(&ticker, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	err = DB.Save(&ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	c.JSON(http.StatusOK, NewJSONSuccessResponse("ticker", NewTickerResponse(&ticker)))
}

//PutTickerUsersHandler changes the allowed users for a ticker
func PutTickerUsersHandler(c *gin.Context) {
	me, err := Me(c)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, ErrorUserNotFound))
		return
	}

	tickerID, err := strconv.Atoi(c.Param("tickerID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	var ticker Ticker
	err = DB.One("ID", tickerID, &ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeNotFound, err.Error()))
		return
	}

	if !me.IsSuperAdmin {
		if !contains(me.Tickers, tickerID) {
			c.JSON(http.StatusForbidden, NewJSONErrorResponse(ErrorCodeInsufficientPermissions, ErrorInsufficientPermissions))
			return
		}
	}

	var body struct {
		Users []int `json:"users" binding:"required"`
	}

	err = c.Bind(&body)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	err = AddUsersToTicker(ticker, body.Users)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	users, _ := FindUsersByTicker(ticker)

	c.JSON(http.StatusOK, NewJSONSuccessResponse("users", NewUsersResponse(users)))
}

//
func PutTickerTwitterHandler(c *gin.Context) {
	me, err := Me(c)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, ErrorUserNotFound))
		return
	}

	tickerID, err := strconv.Atoi(c.Param("tickerID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	if !me.IsSuperAdmin {
		if !contains(me.Tickers, tickerID) {
			c.JSON(http.StatusForbidden, NewJSONErrorResponse(ErrorCodeInsufficientPermissions, ErrorInsufficientPermissions))
			return
		}
	}

	var ticker Ticker
	err = DB.One("ID", tickerID, &ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	var body struct {
		Active     bool   `json:"active,omitempty"`
		Disconnect bool   `json:"disconnect"`
		Token      string `json:"token,omitempty"`
		Secret     string `json:"secret,omitempty"`
	}

	err = c.Bind(&body)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	if body.Disconnect {
		ticker.Twitter.Token = ""
		ticker.Twitter.Secret = ""
		ticker.Twitter.Active = false
		ticker.Twitter.User = twitter.User{}
	} else {
		if body.Token != "" {
			ticker.Twitter.Token = body.Token
		}
		if body.Secret != "" {
			ticker.Twitter.Secret = body.Secret
		}
		ticker.Twitter.Active = body.Active
	}

	if ticker.Twitter.Connected() {
		user, err := bridge.Twitter.User(ticker)
		if err == nil {
			ticker.Twitter.User = *user
		}
	}

	err = DB.Save(&ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	c.JSON(http.StatusOK, NewJSONSuccessResponse("ticker", NewTickerResponse(&ticker)))
}

//DeleteTickerHandler deletes a existing Ticker
func DeleteTickerHandler(c *gin.Context) {
	if !IsAdmin(c) {
		c.JSON(http.StatusForbidden, NewJSONErrorResponse(ErrorCodeInsufficientPermissions, ErrorInsufficientPermissions))
		return
	}

	var ticker Ticker
	tickerID, err := strconv.Atoi(c.Param("tickerID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	err = DB.One("ID", tickerID, &ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeNotFound, err.Error()))
		return
	}

	DB.Select(q.Eq("Ticker", tickerID)).Delete(new(Message))
	DB.Select(q.Eq("ID", tickerID)).Delete(new(Ticker))

	c.JSON(http.StatusOK, gin.H{
		"data":   nil,
		"status": ResponseSuccess,
		"error":  nil,
	})
}

//DeleteTickerUserHandler removes ticker credentials for a user
func DeleteTickerUserHandler(c *gin.Context) {
	me, err := Me(c)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, ErrorUserNotFound))
		return
	}

	tickerID, err := strconv.Atoi(c.Param("tickerID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	var ticker Ticker
	err = DB.One("ID", tickerID, &ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeNotFound, err.Error()))
		return
	}

	if !me.IsSuperAdmin {
		if !contains(me.Tickers, tickerID) {
			c.JSON(http.StatusForbidden, NewJSONErrorResponse(ErrorCodeInsufficientPermissions, ErrorInsufficientPermissions))
			return
		}
	}

	userID, err := strconv.Atoi(c.Param("userID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	var user User
	err = DB.One("ID", userID, &user)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeNotFound, err.Error()))
		return
	}

	err = RemoveTickerFromUser(ticker, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	users, _ := FindUsersByTicker(ticker)

	c.JSON(http.StatusOK, NewJSONSuccessResponse("users", NewUsersResponse(users)))
}

func ResetTickerHandler(c *gin.Context) {
	if !IsAdmin(c) {
		c.JSON(http.StatusForbidden, NewJSONErrorResponse(ErrorCodeInsufficientPermissions, ErrorInsufficientPermissions))
		return
	}

	var ticker Ticker
	tickerID, err := strconv.Atoi(c.Param("tickerID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	err = DB.One("ID", tickerID, &ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeNotFound, err.Error()))
		return
	}

	//Delete all messages for ticker
	DB.Select(q.Eq("Ticker", tickerID)).Delete(new(Message))

	ticker.Reset()

	err = DB.Save(&ticker)
	if err != nil {
		c.JSON(http.StatusNotFound, NewJSONErrorResponse(ErrorCodeDefault, err.Error()))
		return
	}

	c.JSON(http.StatusOK, NewJSONSuccessResponse("ticker", NewTickerResponse(&ticker)))
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func updateTicker(t *Ticker, c *gin.Context) error {
	var body struct {
		Domain      string   `json:"domain" binding:"required"`
		Title       string   `json:"title" binding:"required"`
		Description string   `json:"description" binding:"required"`
		Active      bool     `json:"active"`
		PrependTime bool     `json:"prepend_time"`
		Hashtags    []string `json:"hashtags"`
		Information struct {
			Author   string `json:"author"`
			URL      string `json:"url"`
			Email    string `json:"email"`
			Twitter  string `json:"twitter"`
			Facebook string `json:"facebook"`
		} `json:"information"`
	}

	err := c.Bind(&body)
	if err != nil {
		return err
	}
	domain := util.Validator(body.Domain)
	domainResult := domain.Required().MinLength(5).Check()
	title := util.Validator(body.Title)
	titleResult := title.Required().MinLength(5).Check()
	description := util.Validator(body.Description)
	descriptionResult := description.Required().MinLength(5).Check()

	author := util.Validator(body.Information.Author)
	authorResult := author.MinLength(3).Check()
	uRL := util.Validator(body.Information.URL)
	uRLResult := uRL.MinLength(5).Check()
	email := util.Validator(body.Information.Email)
	emailResult := email.IsEmail().Check()
	twitter := util.Validator(body.Information.Twitter)
	twitterResult := twitter.MinLength(5).Check()
	facebook := util.Validator(body.Information.Facebook)
	facebookResult := facebook.MinLength(5).Check()

	if !domainResult {
		return errors.New("Domain: " + domain.E)
	}
	if !titleResult {
		return errors.New("Title: " + title.E)
	}
	if !descriptionResult {
		return errors.New("Description: " + description.E)
	}
	if !authorResult {
		return errors.New("Author: " + author.E)
	}
	if !uRLResult {
		return errors.New("Url: " + uRL.E)
	}
	if !emailResult {
		return errors.New("Email: " + email.E)
	}
	if !twitterResult {
		return errors.New("Twitter: " + twitter.E)
	}
	if !facebookResult {
		return errors.New("Facebook: " + facebook.E)
	}
	if err != nil == true {
		return err
	}
	t.Domain = body.Domain
	t.Title = body.Title
	t.Description = body.Description
	t.Active = body.Active
	t.PrependTime = body.PrependTime
	t.Hashtags = body.Hashtags
	t.Information.Author = body.Information.Author
	t.Information.URL = body.Information.URL
	t.Information.Email = body.Information.Email
	t.Information.Twitter = body.Information.Twitter
	t.Information.Facebook = body.Information.Facebook

	return nil
}
