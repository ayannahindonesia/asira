package handlers

import (
	"asira_borrower/models"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo"
)

func ClientBankServices(c echo.Context) error {
	defer c.Request().Body.Close()

	bankService := models.Service{}

	// pagination parameters
	rows, err := strconv.Atoi(c.QueryParam("rows"))
	page, err := strconv.Atoi(c.QueryParam("page"))
	orderby := strings.Split(c.QueryParam("orderby"), ",")
	sort := strings.Split(c.QueryParam("sort"), ",")

	// filters
	type Filter struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	result, err := bankService.PagedFindFilter(page, rows, orderby, sort, &Filter{
		Name:   c.QueryParam("name"),
		Status: c.QueryParam("status"),
	})

	if err != nil {
		return returnInvalidResponse(http.StatusInternalServerError, err, "query result error")
	}

	return c.JSON(http.StatusOK, result)
}

func ClientBankServicebyID(c echo.Context) error {
	defer c.Request().Body.Close()

	bankService := models.Service{}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	err := bankService.FindbyID(id)
	if err != nil {
		return returnInvalidResponse(http.StatusInternalServerError, err, "bank tidak ditemukan")
	}

	return c.JSON(http.StatusOK, bankService)
}
