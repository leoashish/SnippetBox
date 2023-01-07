package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/leoashish99/snippetBox/internal/models"
	"github.com/leoashish99/snippetBox/internal/validator"
)

func (app *application) home(w http.ResponseWriter, req *http.Request) {

	snippets, err := app.snippets.Latest()

	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w)
		} else {
			app.serverError(w, err)
		}
	}

	data := app.newTemplateData(req)
	data.Snippets = snippets
	app.render(w, http.StatusOK, "home.html", data)

}

func (app *application) snippetView(w http.ResponseWriter, req *http.Request) {
	params := httprouter.ParamsFromContext(req.Context())
	id, err := strconv.Atoi(params.ByName("id"))

	if err != nil || id < 1 {
		app.notFound(w)
		return
	}
	snippet, err := app.snippets.Get(id)

	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w)
		} else {
			app.serverError(w, err)
		}

	}
	data := app.newTemplateData(req)
	data.Snippet = snippet
	app.render(w, http.StatusOK, "view.html", data)

}
func (app *application) snippetCreate(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)

	data.Form = snippetCreateForm{
		Expires: 365,
	}
	app.render(w, http.StatusOK, "create.html", data)
}

type snippetCreateForm struct {
	Title       string
	Content     string
	Expires     int
	FieldErrors map[string]string
	validator.Validator
}

func (app *application) snippetCreatePost(w http.ResponseWriter, req *http.Request) {

	err := req.ParseForm()
	if err != nil {
		return
	}

	expires, err := strconv.Atoi(req.PostForm.Get("expires"))
	if err != nil {
		return
	}

	snippetForm := &snippetCreateForm{
		Title:   req.PostForm.Get("title"),
		Content: req.PostForm.Get("content"),
		Expires: expires,
	}
	snippetForm.CheckField(validator.NotBlank(snippetForm.Title), "title", "This field cannot be blank")
	snippetForm.CheckField(validator.MaxChars(snippetForm.Title, 100), "title", "This field cannnot be greater than 100 characters long.")
	snippetForm.CheckField(validator.NotBlank(snippetForm.Content), "content", "This field cannot be blank")
	snippetForm.CheckField(validator.PermittedInt(snippetForm.Expires, 1, 7, 365), "expires", "This field must equal 1, 7 or 365")

	if !snippetForm.Valid() {
		data := app.newTemplateData(req)
		data.Form = snippetForm
		app.render(w, http.StatusUnprocessableEntity, "create.html", data)
		return
	}
	id, err := app.snippets.Insert(snippetForm.Title, snippetForm.Content, snippetForm.Expires)

	if err != nil {
		app.serverError(w, err)
		return
	}
	app.sessionManager.Put(req.Context(), "flash", "Snippet successfully created!!!")

	http.Redirect(w, req, fmt.Sprintf("/snippet/view/%d", id), http.StatusSeeOther)
}

type userSignUpForm struct {
	Name     string
	Email    string
	Password string
	validator.Validator
}

func (app *application) userSignup(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)

	data.Form = userSignUpForm{}
	app.render(w, http.StatusOK, "signup.html", data)
}
func (app *application) userSignupPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()

	if err != nil {
		return
	}
	form := &userSignUpForm{
		Name:     r.PostForm.Get("name"),
		Email:    r.PostForm.Get("email"),
		Password: r.PostForm.Get("password"),
	}

	// Validate the form contents using our helper functions.
	form.CheckField(validator.NotBlank(form.Name), "name", "This field cannot be blank")
	form.CheckField(validator.NotBlank(form.Email), "email", "This field cannot be blank")
	form.CheckField(validator.Matches(form.Email, validator.EmailRX), "email", "This field must be a valid email address")
	form.CheckField(validator.NotBlank(form.Password), "password", "This field cannot be blank")
	form.CheckField(validator.MinChars(form.Password, 8), "password", "This field must be at least 8 characters long")

	if !form.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, http.StatusOK, "signup.html", data)
	}
	err = app.users.Insert(form.Name, form.Email, form.Password)

	if err != nil {
		if errors.Is(err, models.ErrDuplicateEmail) {
			form.AddFieldError("email", "Email address is already in use")

			data := app.newTemplateData(r)
			data.Form = form
			app.render(w, http.StatusUnprocessableEntity, "signup.html", data)
		} else {
			app.serverError(w, err)
		}
		return
	}
	app.sessionManager.Put(r.Context(), "flash", "Your Signup was successful. Please log in!!")
	http.Redirect(w, r, "/user/login", http.StatusSeeOther)
}

type userLoginForm struct {
	Email    string
	Password string
	validator.Validator
}

func (app *application) userLogin(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Form = &userLoginForm{}
	app.render(w, http.StatusOK, "login.html", data)
}
func (app *application) userLoginPost(w http.ResponseWriter, r *http.Request) {
	var form *userLoginForm

	err := r.ParseForm()

	if err != nil {
		return
	}

	form = &userLoginForm{
		Email:    r.PostForm.Get("email"),
		Password: r.PostFormValue("password"),
	}

	id, err := app.users.Authenticate(form.Email, form.Password)

	if err != nil {
		if errors.Is(err, models.ErrInvalidCredentials) {
			form.AddNonFieldErrors("Email or password is incorrect")

			data := app.newTemplateData(r)
			data.Form = form
			app.render(w, http.StatusUnprocessableEntity, "login.html", data)
		} else {
			app.serverError(w, err)
		}
		return
	}

	err = app.sessionManager.RenewToken(r.Context())

	if err != nil {
		app.serverError(w, err)
	}

	app.sessionManager.Put(r.Context(), "authenticatedUserID", id)

	http.Redirect(w, r, "/snippet/create", http.StatusSeeOther)
}
func (app *application) userLogoutPost(w http.ResponseWriter, r *http.Request) {
	err := app.sessionManager.RenewToken(r.Context())

	if err != nil {
		app.serverError(w, err)
		return
	}

	app.sessionManager.Remove(r.Context(), "authenticatedUserID")

	app.sessionManager.Put(r.Context(), "flash", "You have been logged out successfully!!")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
