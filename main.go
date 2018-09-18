package main

import (
	"bytes"
	"context"
	"crypto/sha512"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/tardigraudio/website/db"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		user := session.Get("user")
		if user == nil {
			// You'd normally redirect to login page
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session token"})
		} else {
			// Continue down the chain to handler etc
			c.Next()
		}
	}
}

func main() {
	context := context.Background()
	port := "8080"

	if len(os.Args) > 1 {
		if matched, _ := regexp.MatchString(`^\d{2,6}$`, os.Args[1]); matched == true {
			port = os.Args[1]
		}
	}

	router := gin.Default()

	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.Sessions("mysession", store))

	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	database, err := db.Open(context, filepath.Join(usr.HomeDir, "/.tardigraudio"))
	if err != nil {
		panic(err)
	}
	defer database.Close()

	router.LoadHTMLGlob("templates/*")
	router.Static("assets/css", "assets/css")

	router.GET("/user/:name", func(c *gin.Context) {
		session := sessions.Default(c)
		username := c.Param("name")

		var user db.User
		user, err := database.GetUser(fmt.Sprintf("%s", session.Get("user")))
		if err != nil {
			log.Fatal(err)
		}

		uploads := []string{}
		c.HTML(http.StatusOK, "user.tmpl", gin.H{
			"username": username,
			"email":    user.Email,
			"uploads":  uploads,
		})
	})

	router.GET("/user/:name/*song", func(c *gin.Context) {
		// session := sessions.Default(c)
		username := c.Param("name")
		song := c.Param("song")
		c.HTML(http.StatusOK, "song.tmpl", gin.H{
			"username": username,
			"song":     song,
		})
	})

	user := router.Group("/useraction")
	user.Use(AuthRequired())
	{
		user.GET("/logout", func(c *gin.Context) {
			session := sessions.Default(c)

			session.Delete("user")
			session.Save()

			c.String(http.StatusOK, "Successfully Logged out")
			router.HandleContext(c)
		})

		user.GET("/upload", func(c *gin.Context) {
			session := sessions.Default(c)

			c.HTML(http.StatusOK, "upload.tmpl", gin.H{
				"currentUser": session.Get("user"),
			})
		})

		user.POST("/upload", func(c *gin.Context) {
			session := sessions.Default(c)
			// single file
			title := c.PostForm("songTitle")
			description := c.PostForm("songDesc")

			var user db.User
			user, err := database.GetUser(fmt.Sprintf("%s", session.Get("user")))
			if err != nil {
				log.Fatal(err)
			}

			file, _ := c.FormFile("file")
			log.Println(file.Filename, title, description)

			// Upload the file to STORJ
			// TODO: Add song to bucket sj://username
			database.AddSong(title, description, user.ID)

			c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", title))
			router.HandleContext(c)
		})
	}
	router.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.tmpl", gin.H{})
	})

	router.POST("/register", func(c *gin.Context) {
		session := sessions.Default(c)
		email := c.PostForm("email")
		username := c.PostForm("username")
		password := c.PostForm("password")

		h := sha512.New()
		h.Write([]byte(password))

		err := database.AddUser(email, username, h.Sum(nil))
		if err != nil {
			log.Println(err)
			c.String(http.StatusInternalServerError, "Failed to register")
			return
		} else {
			// TODO: Create bucket for user with the same name as the user sj://username
			session.Set("user", username)
			session.Save()
			c.String(http.StatusOK, fmt.Sprintf("'%s' registered!", username))
		}
	})

	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.tmpl", gin.H{})
	})

	router.POST("/login", func(c *gin.Context) {
		session := sessions.Default(c)

		username := c.PostForm("username")
		password := c.PostForm("password")

		h := sha512.New()
		h.Write([]byte(password))
		hash := h.Sum(nil)

		user, err := database.GetUser(username)
		if err != nil {
			c.String(http.StatusInternalServerError, "Invalid username or password")
			return
		}

		if !bytes.Equal(hash, user.Hash) {
			c.String(http.StatusInternalServerError, "Invalid username or password")
			return
		}

		session.Set("user", username)
		session.Save()

		c.String(http.StatusOK, fmt.Sprintf("'%s' logged in!", username))
	})

	router.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		var popular []string
		var recent []string

		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"title":       "Tardigraud.io",
			"popular":     popular,
			"recent":      recent,
			"currentUser": session.Get("user"),
		})
	})

	router.Run(fmt.Sprintf(":%s", port))
}
