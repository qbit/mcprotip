package main

import (
	"encoding/json"
	"github.com/fzzy/radix/redis"
	"github.com/gorilla/mux"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
)

const mcTemplate = `
<html>
<head>
<title>MCProTip</title>
<script src="//code.jquery.com/jquery-2.1.3.min.js"></script>
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.2/css/bootstrap.min.css">
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.2/css/bootstrap-theme.min.css">
<link rel="stylesheet" href="//maxcdn.bootstrapcdn.com/font-awesome/4.3.0/css/font-awesome.min.css">
<script src="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.2/js/bootstrap.min.js"></script>
<style>
body {
  width: 500px;
}

i {
  padding: 3px;
}
</style>
</head>
<body>
<div>
<table id="tips" class="table table-striped">
<thead>
  <th>ID</th>
  <th>Tip</th>
  <th>Votes</th>
</thead>
{{range .}}
<tr>
   <td>{{.ID}}</td>
   <td>{{.Tip}}</td>
   <td><i id="{{.ID}}_up" class="fa fa-thumbs-o-up"></i><span id="{{.ID}}">{{.Votes}}</span><i id="{{.ID}}_down" class="fa fa-thumbs-o-down"></i></td>
</tr>
{{end}}
</table>
</div>
<script>
function vote(id, v, fn) {
  $.ajax({
    type: "POST",
    url: "/vote",
    data: JSON.stringify({ID: id, Vote: v}),
    success: fn,
    dataType: "json"
  });
}

function setVal(v) {
   console.log(v);
   var e = $('#' + v.ID);
   var val = parseInt(e.text());

   if (v.Vote) {
     val = val + 1;
   } else {
     val = val - 1;
   }
   e.text(val);
}

function vval(v) {
  if (v.match(/^up$/)) {
    return true;
  } else {
    return false;
  }
}

function getVals(e) {
  var id = parseInt(e.attr('id').split(/_/)[0], 10);
  var v = vval(e.attr('id').split(/_/)[1]);

  return [id, v];
}

$('.fa-thumbs-o-down').click(function() {
  var vs = getVals($(this));
  vote(vs[0], vs[1], function(data) {
    console.log("negavoted");
    setVal(data);
  });
});

$('.fa-thumbs-o-up').click(function() {
  var vs = getVals($(this));
  vote(vs[0], vs[1], function(data) {
    console.log("posivoted");
    setVal(data);
  });

});
</script>
</body>
</html>
`

type tip struct {
	ID    int
	Tip   string
	Votes int
}

type votet struct {
	ID   int
	Vote bool
}

type tips []*tip

func (a tips) Len() int           { return len(a) }
func (a tips) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a tips) Less(i, j int) bool { return a[i].Votes > a[j].Votes }

var templ = template.Must(template.New("mcprotip").Parse(mcTemplate))

func getTips() (tips, error) {
	var proTips = tips{}

	client, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		log.Fatal("Can't connect to redis!")
	}

	list := client.Cmd("smembers", "protip")
	proList, err := list.List()
	if err != nil {
		log.Fatal("Can't list the list!")
	}

	defer client.Close()

	for key := range proList {
		var t = tip{}
		t.ID = key
		t.Tip = proList[key]
		t.Votes = getVotes(key, client)
		proTips = append(proTips, &t)
	}

	sort.Sort(tips(proTips))
	return proTips, err
}

func setVote(id int, state bool) int {
	val := 0
	if state {
		val = 1
	} else {
		val = -1
	}

	client, err := redis.Dial("tcp", "localhost:6379")

	defer client.Close()

	if err != nil {
		log.Fatal("Can't connect to redis!")
	}

	r := client.Cmd("HINCRBY", "protip_votes", id, val)
	if r.Err != nil {
		log.Fatalf("%v", r.Err)
	}

	return val
}

func getVotes(id int, client *redis.Client) int {
	votes := client.Cmd("hget", "protip_votes", id)
	if votes.Err != nil {
		log.Fatalf("%v", votes.Err)
	}

	ret, _ := votes.Int()

	return ret
}

func vote(w http.ResponseWriter, req *http.Request) {
	var v = votet{}

	body, err := ioutil.ReadAll(io.LimitReader(req.Body, 1048576))
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	if err := req.Body.Close(); err != nil {
		log.Fatalf("err %v", err)
	}

	if err := json.Unmarshal(body, &v); err != nil {
		log.Fatalf("no tip for you! %v", err)
	}

	log.Printf("%d, %v", v.ID, v.Vote)

	_ = setVote(v.ID, v.Vote)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Fatal("can't encode vote!")
	}

}

func showTips(w http.ResponseWriter, req *http.Request) {
	log.Print("showing tips")
	ts, err := getTips()
	if err != nil {
		log.Fatalf("Can't get tips!")
	}

	templ.Execute(w, ts)
}

func showJSONTips(w http.ResponseWriter, req *http.Request) {
	log.Print("showing JSON tips")
	ts, err := getTips()
	if err != nil {
		log.Fatalf("Can't get tips")
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(ts); err != nil {
		log.Fatalf("can't encode tips!")
	}
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/vote", vote).Methods("POST")
	r.HandleFunc("/", showTips)
	r.HandleFunc("/json", showJSONTips)

	http.Handle("/", r)
	http.ListenAndServe(":3016", nil)
}
