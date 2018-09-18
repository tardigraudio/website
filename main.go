package main

import (
	"context"
  "crypto/sha512"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/tardigraudio/website/db"
  "github.com/gin-contrib/sessions"
  "github.com/gin-contrib/sessions/cookie"
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

	router.GET("/user/:name", func(c *gin.Context) {
		username := c.Param("name")
    email := ""
    uploads := []string{}
		c.HTML(http.StatusOK, "user.tmpl", gin.H{
      "username": username,
      "email": email,
      "uploads": uploads,
    })
	})

	router.GET("/user/:name/*song", func(c *gin.Context) {
		username := c.Param("name")
		song := c.Param("song")
		c.HTML(http.StatusOK, "song.tmpl", gin.H{
      "username": username,
      "song": song,
    })
	})

  private := router.Group("/upload")
  {
    private.GET("/upload", func(c *gin.Context) {
  		c.HTML(http.StatusOK, "upload.tmpl", gin.H{})
  	})

  	private.POST("/upload", func(c *gin.Context) {
  		// single file
  		title := c.PostForm("songTitle")
  		description := c.PostForm("songDesc")
      user_id := 0

  		file, _ := c.FormFile("file")
  		log.Println(file.Filename, title, description)

  		// Upload the file to STORJ
  		database.AddSong(title, description, user_id)

  		c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", file.Filename))
  	})
	}

  private.Use(AuthRequired())

  router.GET("/register", func(c *gin.Context) {
    c.HTML(http.StatusOK, "register.tmpl", gin.H{})
  })

  router.POST("/register", func(c *gin.Context) {
    session := sessions.Default(c)
    // single file
    email := c.PostForm("email")
    username := c.PostForm("username")
    password := c.PostForm("password")

    h := sha512.New()
    h.Write([]byte(password))

    err := database.AddUser(email, username, h.Sum(nil))
    if err != nil {
      log.Println(err)
      c.String(http.StatusInternalServerError, "Failed to register")
    } else {
      session.Set("user", email)
      session.Save()
      c.String(http.StatusOK, fmt.Sprintf("'%s' registered!", email))

    }
  })

	router.GET("/login", func(c *gin.Context) {
		c.String(http.StatusOK, "login")
	})

	router.GET("/", func(c *gin.Context) {
		var popular []string
		var recent []string
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"title":   "Tardigraud.io",
			"popular": popular,
			"recent":  recent,
		})
	})

	router.Run(fmt.Sprintf(":%s", port))
}
