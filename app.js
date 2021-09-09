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

String.prototype.trimRight = function(charlist) {
  if (charlist === undefined)
    charlist = "\s";

  return this.replace(new RegExp("[" + charlist + "]+$"), "");
};

const myapp = {
  data() {
    return {
      path: _pathname,
      dir: _getDir(_pathname).trimRight("/"),
      hash: [],
      response: {},
      select: {},
      files: [],
      desc: "1",
    }
  },
  mounted() {
    if (this.path === "/") {
      this.dir = this.path;
    }
    console.log(this.path);
    console.log(this.dir);
    this.listApi(this.path);
  },
  methods: {
    clickDir(path, file, hash) {
      file = file.trimRight("/");
      path = path.trimRight("/");
      this.path = path + "/" + file
      this.listApi(this.path);
      var nextURL = _host + this.path;
      var nextTitle = 'My new page title';
      var nextState = {
        additionalInformation: 'Updated the URL with JS'
      };
      window.history.pushState(nextState, nextTitle, nextURL);
      this.dir = _getDir(this.path);
      console.log(hash);
      this.hash.push(hash);
      console.log(this.hash);
    },
    clickUpDir(path) {
      this.path = _upDir(path);
      this.listApi(this.path);
      var nextURL = _host + this.path;
      var nextTitle = 'My new page title';
      var nextState = {
        additionalInformation: 'Updated the URL with JS'
      };
      window.history.pushState(nextState, nextTitle, nextURL);
      this.dir = _getDir(this.path);
      //this.hash.pop();
      console.log(this.hash);
      //       this.scrollMeTo(this.hash); 
    },
    onSelect(file) {
      console.log(this.select);
    },
    checkTextClass(file) {
      if (this.hash[this.hash.length - 1] === file.Hash) {
        return "text-warning"
      }
      if (this.hash.includes(file.Hash)) {
        return "text-danger"
      }
      return "text"
    },
    operation(action) {
      var data = {};
      data.files = this.select;
      data.dir = this.path;
      data.action = action;
      axios.post("/api?action=operation", data)
        .then(response => {
          console.log(response.data);
        })
        .catch(error => {
          console.log(error)
        })
      console.log(action);
      this.select = {};
      this.listApi(this.path);
    },
    listApi(path) {
      axios.get("/api?action=list&path=" + path)
        .then(response => {
          this.response = response.data;
          this.files = response.data.Data.Files;
          console.log(response.data);
        })
        .catch(error => {
          console.log(error)
        })
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
    scrollMeTo(refName) {
      var element = this.$refs[refName];
      console.log(element)
      var top = element.offsetTop;
      window.scrollTo(0, top);
    },
  },
}

const app = Vue.createApp(myapp);
app.mount('#app');
