package main

import (
  "database/sql"
  "html/template"
  "log"
  "net/http"
  //"encoding/base64"
  "io/ioutil"

  _ "github.com/go-sql-driver/mysql"
  "github.com/hashicorp/vault/api"

)

var db *sql.DB
var tpl *template.Template
var vlt *api.Client

const KEY_NAME = "my_app_key"

type user struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
  Address   string
  Files     []user_file
  FileNames string
  Datakey   string
}

type user_file struct {
  ID       int64
  UserID   int64
  Mimetype string
  FileName string
  File     []byte
}

func initVaultClient() *api.Client {
	cfg := api.DefaultConfig()
	cfg.Address = "http://127.0.0.1:8200"

	c, err := api.NewClient(cfg)
	if err != nil {
		log.Fatalln(err)
	}

	return c
}

func initDB() *sql.DB {
  dsn := "vault:vaultpw@tcp(127.0.0.1:3306)/my_app"
  db, err := sql.Open("mysql", dsn)

  if err != nil {
    panic(err)
  }

  err = db.Ping()
  if err != nil {
  	log.Fatalln(err)
  }

  create_tables(db)

  return db
}

func create_tables(db *sql.DB) {
  _, err := db.Exec("USE my_app")
  if err != nil {
    log.Fatalln(err)
  }

  create_user_table :=
    "CREATE TABLE IF NOT EXISTS `user_data`(" +
    "`user_id` INT(11) NOT NULL AUTO_INCREMENT, " +
    "`user_name` VARCHAR(256) NOT NULL," +
    "`first_name` VARCHAR(256) NULL, " +
    "`last_name` VARCHAR(256) NULL, " +
    "`address` VARCHAR(256) NOT NULL, " +
    "`datakey` VARCHAR(256) DEFAULT NULL, " +
    "PRIMARY KEY (user_id) " +
    ") engine=InnoDB;"

  log.Println("Creating user table (if not exist)")

  _, err = db.Exec(create_user_table)
  if err != nil {
    log.Fatalln(err)
  }

  log.Println("Creating files table (if not exist)")

  create_files_table :=
    "CREATE TABLE IF NOT EXISTS `user_files`( " +
    "`file_id` INT(11) NOT NULL AUTO_INCREMENT, " +
    "`user_id` INT(11) NOT NULL, " +
    "`mime_type` VARCHAR(256) DEFAULT NULL, " +
    "`file_name` VARCHAR(256) DEFAULT NULL, " +
    "`file` LONGBLOB DEFAULT NULL, " +
    "PRIMARY KEY (file_id) " +
    ") engine=InnoDB;"

    _, err = db.Exec(create_files_table)
    if err != nil {
      log.Fatalln(err)
    }
}

func initTemplates() {
  tpl = template.Must(template.ParseGlob("templates/*"))
}

func main() {
  db = initDB()
  defer db.Close()
  initTemplates()

  vlt = initVaultClient()

  log.Println(vlt.Sys().SealStatus())
  secret, err := getDataKey()
  if err != nil {
    log.Printf("Error getting datakey: %s", err)
  }

  if secret == nil {
    log.Println("No secret retrieved.")
  }
  log.Println(secret.Data["ciphertext"])

  url := "0.0.0.0:1234" // Listen on all interfaces

  // set up routes
  http.Handle("/favicon.ico", http.NotFoundHandler())
  http.HandleFunc("/", indexHandler)
  http.HandleFunc("/view", viewHandler)
  http.HandleFunc("/create", createHandler)
  http.HandleFunc("/update", updateHandler)
  http.HandleFunc("/createRecord", createRecordHandler)

  // run the server
  log.Printf("Server is running at http://%s", url)
  log.Fatalln(http.ListenAndServe(url, nil))
}

func indexHandler(w http.ResponseWriter, req *http.Request) {
  err := tpl.ExecuteTemplate(w, "index.html", nil)
  if err != nil {
    log.Fatalln(err)
  }
}

func createHandler(w http.ResponseWriter, req *http.Request) {
  err := tpl.ExecuteTemplate(w, "create.html", nil)
  if err != nil {
    log.Fatalln(err)
  }
}

func viewHandler(w http.ResponseWriter, req *http.Request) {
  err := tpl.ExecuteTemplate(w, "view.html", getUsers(10))
  if err != nil {
    log.Fatalln(err)
  }
}

func updateHandler(w http.ResponseWriter, req *http.Request) {
  err := tpl.ExecuteTemplate(w, "update.html", nil)
  if err != nil {
    log.Fatalln(err)
  }
}

func getUsers(limit int) []user {
  /*rows, err := db.Query(
		`SELECT user_id,
			user_name,
			first_name,
			last_name,
			address
		FROM user_data LIMIT ?;`, limit)*/

  rows, err := db.Query(
		`SELECT ud.user_id,
            ud.user_name,
            ud.first_name,
            ud.last_name,
            ud.address,
            uf.file_name
     FROM user_data AS ud, user_files AS uf
     WHERE ud.user_id=uf.user_id LIMIT ?;`, limit)

  if err != nil {
    log.Println(err)
  }

  log.Println(rows)

  users := make([]user, 0, 10)
  for rows.Next() {
		usr := user{}
		rows.Scan(&usr.ID, &usr.Username, &usr.FirstName, &usr.LastName, &usr.Address, &usr.FileNames)
		users = append(users, usr)
	}
	log.Println(users)

  return users
}

func getUserByName(username, firstname, lastname string) user {
  var usr user
  rows, err := db.Query(`SELECT user_id, user_name, first_name, last_name, address
    FROM users
    WHERE user_name = ?
    AND first_name = ?
    AND last_name = ?`,
    username, firstname, lastname)
  if err != nil {
  	log.Fatal(err)
  }
  defer rows.Close()
  for rows.Next() {
    usr := user{}
    rows.Scan(&usr.ID, &usr.Username, &usr.FirstName, &usr.LastName, &usr.Address)
  }
  err = rows.Err()
  if err != nil {
  	log.Fatal(err)
  }
  return usr
}

func getUserId(username, firstname, lastname string) int64 {
  rows, err := db.Query("SELECT user_id FROM users WHERE user_name = ? AND first_name = ? AND last_name = ?", username, firstname, lastname)
  if err != nil {
  	log.Fatal(err)
  }

  defer rows.Close()

  usr := user{}
  for rows.Next() {
  	err := rows.Scan(&usr.ID)
  	if err != nil {
  		log.Fatal(err)
  	}
  }

  err = rows.Err()
  if err != nil {
  	log.Fatal(err)
  }

  return usr.ID
}

// form endpoint
func createRecordHandler(w http.ResponseWriter, req *http.Request) {
  if req.Method == http.MethodPost {
    var err error
    usr := user{}
		usr.Username = req.FormValue("username")
		usr.FirstName = req.FormValue("firstname")
		usr.LastName = req.FormValue("lastname")
    usr.Address = req.FormValue("address")

    secret, err := getDataKey()
    if err != nil {
      log.Println(err)
    }

    // retrieve ciphertext to save, plaintext to encrypt files
    ciphertext, plaintext := secret.Data["ciphertext"], secret.Data["plaintext"]

    log.Printf("Secret ciphertext: %s, plaintext: %s", ciphertext, plaintext)

    result, err := db.Exec(
			"INSERT INTO user_data (user_name, first_name, last_name, address, datakey) VALUES (?, ?, ?, ?, ?)",
			usr.Username,
			usr.FirstName,
			usr.LastName,
			usr.Address,
      ciphertext,
		)

    if err != nil {
      log.Println(err)
    }

    file, handler, err := req.FormFile("userfile")
    if err == nil && file != nil && handler != nil {
      log.Printf("Found a formfile: %s with headers: %s", handler.Filename, handler.Header.Get("Content-Type"))
      if err != nil {
        log.Println(err)
      }

      filedata, err := ioutil.ReadAll(file)

      if err != nil {
        log.Printf("Error reading file data: %s", err)
      }

      user_id, err := result.LastInsertId()
      if err != nil {
        log.Println(err)
      }

      _, err = db.Exec(
        "INSERT INTO `user_files` (`user_id`, `mime_type`, `file_name`, `file`) VALUES (?, ?, ?, ?)",
        user_id,
        handler.Header.Get("Content-Type"), // need user_id
        handler.Filename,
        filedata,
      )
      defer file.Close()
      if err != nil {
        log.Printf("Error saving file: %s", err)
      }
    } else {
      log.Printf("Error retrieving file: %s", err)
    }

    log.Printf("Saved from form: Username: %s, FirstName: %s, LastName: %s, Address: %s", usr.Username, usr.FirstName, usr.LastName, usr.Address)
		err = tpl.ExecuteTemplate(w, "create.html", map[string]interface{} {
      "success": true,
      "username": usr.Username,
    })

    if err != nil {
      log.Println(err)
    }
		return
	}
	http.Error(w, "Method Not Supported", http.StatusMethodNotAllowed)
}

func getDataKey() (*api.Secret, error) {
  datakey, err := vlt.Logical().Write("transit/datakey/plaintext/" + KEY_NAME, nil)
  return datakey, err
}
