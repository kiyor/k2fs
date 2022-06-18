var _pathname = window.location.pathname; // Returns path only (/path/example.html)
var _url = window.location.href; // Returns full URL (https://example.com/path/example.html)
var _origin = window.location.origin; // Returns base URL (https://example.com)

var _getDir = function(path) {
  var p = path.split(/[\/]+/);
  var dir = p[p.length - 1];
  if (dir.length === 0) {
    dir = p[p.length - 2];
  }
  return dir
}

var _upDir = function(path) {
  var p = path.split(/[\/]+/);
  p.pop();
  console.log(p);
  return p.join('/')
}

var _hide = function() {
  var myOffcanvas = document.getElementById('offcanvas');
  var bsOffcanvas = new bootstrap.Offcanvas(myOffcanvas);
  bsOffcanvas.toggle();
  bsOffcanvas.toggle();
  bsOffcanvas.toggle();
  bsOffcanvas.toggle();
}

var _show = function() {
  var myOffcanvas = document.getElementById('offcanvas');
  var bsOffcanvas = new bootstrap.Offcanvas(myOffcanvas);
  bsOffcanvas.hide();
}

var _jump = function(h) {
  let a = document.getElementById(h)
  if (a != undefined) {
    let top = a.offsetTop; //Getting Y of target element
    window.scrollTo(0, top); //Go there directly or some transition
  }
}

var _getQS = function(param) {
  var sPageURL = window.location.search.substring(1),
    sURLVariables = sPageURL.split(/[&||?]/),
    res;
  for (var i = 0; i < sURLVariables.length; i += 1) {
    var paramName = sURLVariables[i],
      sParameterName = (paramName || '').split('=');

    if (sParameterName[0] === param) {
      res = sParameterName[1];
    }
  }
  return res;
}

String.prototype.trimRight = function(charlist) {
  if (charlist === undefined)
    charlist = "\s";

  return this.replace(new RegExp("[" + charlist + "]+$"), "");
};

window.onload = function() {
  document.getElementsByClassName("blink_me").fadeOut(3000).fadeIn(3000, blink);
}

const myapp = {
  data() {
    return {
      class_container: "container-lg",
      path: _pathname,
      dir: "",
      df: [],
      updir: "",
      resp: {},
      select: {},
      files: [],
      subListOpen: {}, // open sub folder, path: bool
      subList: {}, // open sub folder, path: files
      labelMap: {}, // backup label color
      lastLabel: "", // the last click folder name
      desc: "1",
      clickCounter: 0,
      clickTimer: null,
      history: [],
    }
  },
  async mounted() {
    this.getDf();
    await this.listApi(this.path);
    var p = this.path.trimRight("/").split("/")
    for (let k in p) {
      if (k > 0) {
        this.history[k - 1] = p[k] + "/";
      }
    }
    var q = _getQS('q');
    if (q != undefined) {
      for (let i = 0; i < this.files.length; i++) {
        if (this.files[i].Name.includes(q)) {
          _jump(this.files[i].Hash);
          this.lastLabel = this.files[i].Name;
          this.colorCleaner();
          break;
        }
      }
    }
    //     console.log(this.history); 
  },
  methods: {
    onClick(path, file) {
      if (!file.IsDir) {
        return
      }

      this.getDf();

      this.clickCounter++;
      if (this.clickCounter === 1) { // single click
        this.clickTimer = setTimeout(() => {
          this.clickSubDir(path, file);
          this.clickCounter = 0;
        }, 300);
      } else {
        clearTimeout(this.clickTimer);
        this.clickDir(path, file);
        this.clickCounter = 0;
      }
    },
    async clickDir(path, file) {
      var sub = this.getSub(path, file.Name);
      var p = sub.split("/")
      for (let k in p) {
        if (k > 0) {
          this.history[k - 1] = p[k] + "/";
        }
      }
      //       console.log(this.history); 
      this.path = sub;
      console.log(1);
      await this.listApi(this.path);
      var nextURL = _host + this.path;
      var nextTitle = '';
      var nextState = {
        additionalInformation: ''
      };
      window.history.pushState(nextState, nextTitle, nextURL);
      console.log(2);
    },
    clickFile(file) {
      this.lastLabel = file.Name;
      this.colorCleaner();
    },
    isOpened(path, file) {
      if (!file.IsDir) {
        return false;
      }
      if (this.subListOpen[this.getSub(path, file.Name)] === undefined) {
        this.subListOpen[this.getSub(path, file.Name)] = false;
      }
      return this.subListOpen[this.getSub(path, file.Name)];
    },
    trimRight(a, b) {
      return a.trimRight(b);
    },
    open(file) {
      console.log(file.ShortCut);
    },
    getSubLink(path, file, sub) { // string path, object file and object sub
      if (sub.IsDir) {
        return path.trimRight('/') + '/' + file.Name + sub.Name;
      } else {
        if (sub.ShortCut.length > 0) {
          return sub.ShortCut;
        } else {
          return '/statics' + path.trimRight('/') + '/' + file.Name + sub.Path;
        }
      }
    },
    dfColor(p) {
      if (p > 95) {
        return "text-danger blink_me"
      } else if (p > 85) {
        return "text-warning blink_me"
      } else {
        return ""
      }
    },
    getDf() {
      axios.get("/api?action=df")
        .then(response => {
          console.log(response.data);
          this.df = response.data;
        })
        .catch(error => {
          console.log(error)
        })
    },
    getLink(path, file) {
      return '/statics' + path + '/' + file.Name;
    },
    getSub(path, file) {
      file = file.trimRight("/");
      path = path.trimRight("/");
      return path + "/" + file;
    },
    clickSubDir(path, file) {
      if (!file.IsDir) {
        return
      }
      var sub = this.getSub(path, file.Name);
      if (this.subListOpen[sub] === undefined) {
        this.subListOpen[sub] = true;
      } else {
        this.subListOpen[sub] = !this.subListOpen[sub]
      }
      this.lastLabel = file.Name;
      //       console.log("lastLabel: " + this.lastLabel); 
      // if make it open, clean up other open folder
      if (this.subListOpen[sub]) {
        this.listSubApi(sub);
        //         file.Meta.Label = "dark"; 
        for (let k in this.subListOpen) {
          if (k !== sub) {
            this.subListOpen[k] = false;
          }
        }
      }
    },
    colorCleaner() {
      // roll back other folder label color exect last folder
      for (let i = 0; i < this.files.length; i++) {
        if (this.lastLabel === this.files[i].Name) {
          this.files[i].Meta.Label = "dark";
        } else if (this.history[this.history.length - 1] === this.files[i].Name) {
          this.files[i].Meta.Label = "dark";
        } else {
          this.files[i].Meta.Label = this.labelMap[this.files[i].Name];
        }
      }
      console.log("colorCleaner finished")
    },
    async clickUpDir() {
      this.path = this.resp.UpDir;
      console.log(1);
      await this.listApi(this.path);
      var nextURL = _host + this.path;
      var nextTitle = '';
      var nextState = {
        additionalInformation: ''
      };
      window.history.pushState(nextState, nextTitle, nextURL);
      //       console.log(this.history); 
      console.log(2);
      this.colorCleaner();
      this.history.pop();
    },
    onSelect() {
      console.log(this.select);
      _show();
    },
    checkTableClass(file) {
      if (file.Meta.Label.length > 0) {
        return "table-" + file.Meta.Label
      }
      return ""
    },
    operation(action) {
      var data = {};
      data.files = this.select;
      data.dir = this.path;
      data.action = action;
      console.log(data);
      axios.post("/api?action=operation", data)
        .then(response => {
          console.log(response.data);
          this.select = {};
          this.listApi(this.path);
          this.getDf();
          _hide();
        })
        .catch(error => {
          console.log(error)
        })
    },
    async listApi(path) {
      var data = {};
      data.path = path;
      data.list = "read";
      await axios.post("/api?action=list", data)
        .then(response => {
          this.resp = response.data.Data;
          this.updir = this.resp.UpDir;
          this.dir = this.resp.Dir;
          this.files = this.resp.Files;
          for (let i = 0; i < this.files.length; i++) {
            this.labelMap[this.files[i].Name] = this.files[i].Meta.Label;
          }
          //           console.log(this.resp); 
          //           console.log(this.labelMap); 
          console.log("listApi finished");
        })
        .catch(error => {
          console.log(error)
        })
      this.colorCleaner();
      var i = this.history[this.history.length - 1];
      console.log("need scrollTo: " + i);
      _jump(i);
    },
    async listSubApi(path) {
      var data = {};
      data.path = path;
      data.listdir = "find";
      await axios.post("/api?action=list", data)
        .then(response => {
          var resp = response.data.Data;
          this.subList[path] = resp.Files;
          //           console.log(this.resp); 
          //           console.log(this.subList); 
          console.log("listSubApi finished");
        })
        .catch(error => {
          console.log(error)
        })
      this.colorCleaner();
    },
    sortByApi(thing) {
      if (this.desc === "1") {
        this.desc = "0";
      } else {
        this.desc = "1";
      }
      axios.get("/api?action=session&sortby=" + thing + "&desc=" + this.desc)
        .then(response => {
          this.listApi(this.path);
          console.log(response.data);
        })
        .catch(error => {
          console.log(error)
        })
    },
  },
}

const app = Vue.createApp(myapp);

app.component('selected', {
  props: ['k', 'v'], // define argument
  template: `<li v-if="v">{{k}}</li>`
})

app.mount('#app');
