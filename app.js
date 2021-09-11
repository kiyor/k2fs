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
  var top = document.getElementById(h).offsetTop; //Getting Y of target element
  window.scrollTo(0, top); //Go there directly or some transition
}

String.prototype.trimRight = function(charlist) {
  if (charlist === undefined)
    charlist = "\s";

  return this.replace(new RegExp("[" + charlist + "]+$"), "");
};

const myapp = {
  data() {
    return {
      class_container: "container-lg",
      path: _pathname,
      dir: "",
      updir: "",
      hash: [],
      resp: {},
      select: {},
      files: [],
      subListOpen: {}, // open sub folder, path: bool
      subList: {}, // open sub folder, path: files
      desc: "1",
    }
  },
  mounted() {
    //     console.log(this.path); 
    //     console.log(this.dir); 
    this.listApi(this.path);
  },
  methods: {
    clickDir(path, file) {
      this.path = this.getSub(path, file.Name);
      this.listApi(this.path);
      var nextURL = _host + this.path;
      var nextTitle = '';
      var nextState = {
        additionalInformation: ''
      };
      window.history.pushState(nextState, nextTitle, nextURL);
      console.log(hash);
      this.hash.push(hash);
      console.log(this.hash);
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
    getSubLink(path, file, sub) { // string path, object file and object sub
      if (sub.IsDir) {
        return path.trimRight('/') + '/' + file.Name + sub.Name;
      } else {
        return '/statics' + path.trimRight('/') + '/' + file.Name + sub.Name;
      }
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
      console.log(sub);
      if (this.subListOpen[sub] === undefined) {
        this.subListOpen[sub] = true;
      } else {
        this.subListOpen[sub] = !this.subListOpen[sub]
      }
      if (this.subListOpen[sub]) {
        this.listSubApi(sub);
      }
    },
    clickUpDir(path) {
      this.path = this.resp.UpDir;
      var hash = this.resp.Hash
      this.listApi(this.path);
      console.log(hash);
      var nextURL = _host + this.path;
      var nextTitle = '';
      var nextState = {
        additionalInformation: ''
      };
      window.history.pushState(nextState, nextTitle, nextURL);
      //       window.location.hash = hash; 
      //       window.location.reload(); 
      console.log(this.hash);
      //       _jump(hash); 
    },
    onSelect(file) {
      console.log(this.select);
      _show();
    },
    checkTextClass(file) {
      if (this.hash[this.hash.length - 1] === file.Hash) {
        return "text-warning"
      }
      if (this.hash.includes(file.Hash)) {
        return "text-danger"
      }
      return ""
    },
    operation(action) {
      var data = {};
      data.files = this.select;
      data.dir = this.path;
      data.action = action;
      axios.post("/api?action=operation", data)
        .then(response => {
          console.log(response.data);
          this.select = {};
          this.listApi(this.path);
          _hide();
        })
        .catch(error => {
          console.log(error)
        })
    },
    listApi(path) {
      var data = {};
      data.path = path;
      axios.post("/api?action=list", data)
        .then(response => {
          this.resp = response.data.Data;
          this.updir = this.resp.UpDir;
          this.dir = this.resp.Dir;
          this.files = this.resp.Files;
          console.log(this.resp);
        })
        .catch(error => {
          console.log(error)
        })
    },
    listSubApi(path) {
      var data = {};
      data.path = path;
      axios.post("/api?action=list", data)
        .then(response => {
          var resp = response.data.Data;
          this.subList[path] = resp.Files;
          console.log(this.resp);
          console.log(this.subList);
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
  },
}

const app = Vue.createApp(myapp);

app.component('selected', {
  props: ['k', 'v'], // define argument
  template: `<li v-if="v">{{k}}</li>`
})

app.component('render-file', {
  props: ['file', 'path', 'select', 'hash'], // define argument
  methods: {
    checkTextClass(file) {
      if (this.hash[this.hash.length - 1] === file.Hash) {
        return "text-warning"
      }
      if (this.hash.includes(file.Hash)) {
        return "text-danger"
      }
      return ""
    },
  },
  template: `
<tr v-bind:class="'table-' + file.Meta.Label">
  <td v-bind:name="file.Hash" v-bind:id="file.Hash">
    <i v-if="file.Meta.Star" class="fas fa-star"></i>
    <span v-if="file.IsDir">
      <i class="far fa-folder-open"></i>
      <a v-on:click="clickDir(path,file.Name,file.Hash)" v-bind:class="checkTextClass(file)"> {{file.Name}}</a>
    </span>
    <span v-if="!file.IsDir">
      <i class="far fa-file"></i>
      <a v-bind:href="'/statics' + path + '/' + file.Name" target="_blank"> {{file.Name}}</a>
    </span>
  </td>
  <td>{{file.SizeH}}</td>
  <td>
    <input type="checkbox" v-model="select[file.Name]" data-bs-toggle="offcanvas" data-bs-target="#offcanvas" aria-controls="offcanvas">
  </td>
  <td>{{file.ModTimeH}}</td>
</tr>
`
})

app.mount('#app');
