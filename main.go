package main

import (
  "database/sql"
  "log"
  "fmt"
  //"strings"
  //"bufio"
  "io"
  "os"
  "net/http"
  _ "github.com/mattn/go-sqlite3"
  "path/filepath"
  //"io/fs"
  //"io/ioutil"
  //"strings"
)

type Pair struct {
  Key string
  Value string
}

const tempdir = "tmp"
const addr = "localhost:8080"

//var tempdir string
//var host string

// I don't think we have to do anything else on this particular function, I think we are good.
func openDatabase(path string) (*sql.DB, error) {
    // the path to the database--this could be an absolute path
  //path = "austen.db"
  options :=
      "?" + "_busy_timeout=10000" +
          "&" + "_case_sensitive_like=OFF" +
          "&" + "_foreign_keys=ON" +
          "&" + "_journal_mode=OFF" +
          "&" + "_locking_mode=NORMAL" +
          "&" + "mode=rw" +
          "&" + "_synchronous=OFF"
  db, err := sql.Open("sqlite3", path+options)
  if err != nil {
      // handle the error here
      log.Fatalf("error opening database", err)
    }
  return db, nil
}

func createDatabase(path string) (*sql.DB, error) {
  // creates a new database and prepares it for use

  // if the database file already exists, delete it before continuing.
  /*
  if path exists {
    path.drop()
  }
  */
  // create table pairs (key text, value text)

  // it's BIG BRAIN TIME, let's just remove the database period whenever we try
  // to create one, then we don't even need to check (:
  os.Remove(path)
  // now we can open teh file
  db, err := openDatabase(path)
  if err != nil {
    log.Fatalf("Database was unable to create with path %s\n", path)
    return nil, err
  }
  // if we get here we can create it! Mega POG!!
  _, err = db.Exec(`create table pairs (key text, value text);`)
  //Exec executes a prepared statement with the given arguments and returns a Result summarizing the effect of the statement.

  return db, nil
}




/*When the master first starts up,
it will need to partition the input data set for the individual
mapper tasks to access. This function is responsible for splitting a
single sqlite database into a fixed number of smaller databases.*/
func splitDatabase(source, outputDir, outputPattern string, m int) ([]string, error) {
/*It is given the pathname of the input database,
and a pattern for creating pathnames for the output files.
The pattern is a format string for fmt.SFatalf to use,
so you might call it using:*/

  // we can start off by using our open database function to start
  db, err := openDatabase(source)
  if err != nil {
    log.Fatalf("split database:", err)
  }
  // defer will execute after we return, that's kinda what defer does,
  //is it delays the operation in defer from running until the function is finished
  defer db.Close()

  count, err := rowCount(db)
  if err != nil {
    log.Fatalf("spliting database query into pairs", err)
  }
  // debug line
  fmt.Println("Count: ", count)
  if count < m {
    log.Fatal("Can't have the count less than m")
  }

  // create a slice of databases
  // a := make([]int, 5)  // len(a)=5
  // set its length equal to the sql.DB's length
  databaseSlice := make([]*sql.DB, 0)
  // name slice, nothing in by default
  names := make([]string, 0)

  // spliting the database into a bunch of different files, as for the mapping stuff to happen
  for i := 0; i < m; i++ {
    // SFatalf(format string, a ...interface{}) string
    filename := filepath.Join(outputDir, fmt.Sprintf(outputPattern, i))

    // creating a new database to put into a directory
    outputdb, err := createDatabase(filename)

    if err != nil {
      log.Fatalf("Connection Error: %v", err)
    }

    defer outputdb.Close()

    // append database and names to their slices
    databaseSlice = append(databaseSlice, outputdb)
    names = append(names, filename)
  }

  // send in sql query to grab keys and pairs
  rows, err := db.Query("select key, value from pairs;")
  if err != nil {
    log.Fatalf("split databases selection:", err)
  }

  defer rows.Close()

  index := 0 // seting an index

  for rows.Next() {
    // get pair from rows
    pair, err := getPair(rows)
    if err != nil {
      log.Fatalf("split database%v", err)
    }
    // inserting the pairs into the seperate databases
    err = insertPair(pair, databaseSlice[index % m])
    if err != nil {
      log.Fatalf("split database%v", err)
    }
    index++
  }
  err = rows.Err()
  if err != nil {
    log.Fatal("end of split counting%v", err)
  }
  return names, nil
}

func mergeDatabases(urls []string, dest string, temp string) (*sql.DB, error) {
  // initially create the database, return an error if its still a problem
	db, err := createDatabase(dest)
	if err != nil {
		return nil, fmt.Errorf("Issue with creating the database: %v", err)
	}


	for _, url := range urls {

		// Try actually downloading
		if err := download(url, temp); err != nil {
			db.Close()
			return nil, fmt.Errorf("Issue with downloading database %s: %v", url, err)
		}
		// try actually merging
		if err := gatherInto(db, temp); err != nil {
			db.Close()
			return nil, fmt.Errorf("Issue with merging the database %s: %v", url, err)
		}
	}

	return db, nil
}

func download(url, path string) error {
  //This function takes a URL and a pathname, and stores the file it downloads from the URL at the given location
  pathway, err := os.Create(path)
  if err != nil {
    log.Fatalf("Download create %s", err)
  }
  // always delay, but be sure we execute the close (:
  defer pathway.Close()

  // actually grabbing the body of the URL (:
  b, err := http.Get(url)
  if err != nil {
    log.Fatalf("Download get %s", err)
  }
  defer b.Body.Close()
  // always return an error code

  _, err = io.Copy(pathway, b.Body)
  if err != nil {
    log.Fatalf("Download opendb", err)
  }
  // utilize net/http

  // nil is generally considered "no error happened"
  return nil
}

func gatherInto(db *sql.DB, path string) error {
  // This function takes an open database (the output file) and does a thing


  query := fmt.Sprintf("attach '%s' as merge;", path)
  _, err := db.Exec(query)

  // the pathname of another database (the input file) and merges
  if err != nil {
    log.Printf("Error executing query: %s try merging better\n", path)
    return err
  }

  // the input file into the output file:

  // slap in an SQL Query

  /*
  attach ? as merge;
  insert into pairs select * from merge.pairs;
  detach merge;
  */

  _, err = db.Exec(`INSERT INTO pairs SELECT * FROM merge.pairs;`)
  if err != nil {
    log.Fatalf("INSERT Query", err)
  }

  _, err = db.Exec(`detach merge;`)
  if err != nil {
    log.Fatalf("Merch detachment", err)
  }

  return nil
}

/*
go func() {
    http.Handle("/data/", http.StripPrefix("/data", http.FileServer(http.Dir(tempdir))))
    if err := http.ListenAndServe(address, nil); err != nil {
        log.Fatalf("Error in HTTP server for %s: %v", myaddress, err)
    }
}()
*/


// counting rows helper function

func rowCount(db *sql.DB) (int, error) {
  // launch a sequel query that counts all of the rows, according to the pairs key
  rows, err := db.Query("select count(1) from pairs;")
  if err != nil {
    return -1, err
  }
  defer rows.Close()

  // the actual counting loop
  count := 0
  for rows.Next() {

    err := rows.Scan(&count)
    if err != nil {
      return -1, err
    }
  }
  // keep in mind that a -1 means we have a bad or unreadable table
  return count, nil
}

// grabbing a pair from an sql row
func getPair(row *sql.Rows) (Pair, error) {
  var key, value string
  err := row.Scan(&key, &value)
  if err != nil {
    return Pair{}, err
  }

  return Pair{key, value}, nil
}

// most of this code I am pulling from CS3200 of all places, where we built databases
//with key value pairs
func insertPair(pair Pair, db *sql.DB) error {
  _, err := db.Exec("INSERT INTO pairs (key, value) VALUES (?, ?);", pair.Key, pair.Value)
  if err != nil {
    log.Fatalf("insertPair", err)
  }
  return nil
}

func main() {

  // I might need to ask a bunch of questions on this part, like where exactly do I start?

  //MapReduce will be a library, but for now it will be easier to test it if
  //you put it in package main and write a main function. Use the
  //Jane Austen example file and run your splitDatabase function
  //to break it into several pieces.
  //Count the number of rows in each piece from the command line using:
  //sqlite3 austen-0.db 'select count(1) from pairs'
  //result is 75811

  //Add the numbers up for each of the pieces and make sure the total
  //matches the total number of rows in the input file.
  log.SetFlags(log.Lshortfile)

  // keep track of our go routines
	go localService(tempdir, addr)

  // check if split is successful
	paths, err := splitDatabase("data/austen.db", tempdir, "output-%d.db", 10)
	if err != nil {
		log.Fatalf("Issue split database: %v", err)
	}

  // change each of the paths to proper url links
	for i := range paths {
		paths[i] = "http://" + addr + "/" + paths[i]
		fmt.Println(paths[i])
	}

  // check if merge is successful
	db, err := mergeDatabases(paths, "merged.db", "temp.db")
	if err != nil {
		log.Fatalf("Issue with merge databbases: %v", err)
	}
	defer db.Close()

	// Check the actual count and print out those results
	var total int
	if err := db.QueryRow("SELECT COUNT(*) AS count FROM pairs").Scan(&total); err != nil || err == sql.ErrNoRows {
		log.Fatalf("Selecting all of count error: %v", err)
	}

	log.Printf("Rows in merged db: %d", total)




  //mergeDatabases("localhost:8080/data/output-1")
  // Make sure it has the correct number of rows as well.

  // Since mergeDatabases expects to download data from the network,
  //you need to run a simple web server.
  // The following code will serve all files in the given directory:





  // In this code tempdir is the directory containing the files to be served,
  //and /data/ is the prefix path that each file should have in its URL.
  //address should be the network address to listen on.
  //You can use localhost:8080 to run it on a single machine.
}

func localService(splitDir, addr string) {
	http.Handle("/"+splitDir+"/", http.StripPrefix("/"+splitDir, http.FileServer(http.Dir(splitDir))))
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Printf("Error in HTTP server for %s: %v", addr, err)
	}
}

func makeURL(host, file string) string {
	return fmt.Sprintf("http://%s/tmp/%s", host, file)
}
