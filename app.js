var _pathname = decodeURI(window.location.pathname); // Returns path only (/path/example.html)
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

var _setQS = function(param, paramVal) {
    var newAdditionalURL = "";
    var tempArray = window.location.toString().split("?");
    //   var baseURL = tempArray[0]; 
    var additionalURL = tempArray[1];
    var temp = "";
    if (additionalURL) {
        tempArray = additionalURL.split("&");
        console.log(tempArray, tempArray.length);
        for (var i = 0; i < tempArray.length; i++) {
            console.log(tempArray[i].split('=')[0]);
            if (tempArray[i].split('=')[0] != param) {
                console.log(tempArray[i].split('=')[0], tempArray[i].split('=')[1]);
                newAdditionalURL += temp + tempArray[i];
                temp = "&";
                console.log(temp);
            }
        }
    }

    var rows_txt = temp + "" + param + "=" + paramVal;
    console.log("?" + newAdditionalURL + rows_txt);
    window.history.replaceState('', '', "?" + newAdditionalURL + rows_txt);
}

var _scale_width = window.innerWidth / 2;
//var _show_width = 400;
var _show_width = window.innerWidth / 2 + "px";
var _left_width = window.innerWidth / 2 + "px";
console.log(_show_width);
console.log(_left_width);

String.prototype.trimRight = function(charlist) {
    if (charlist === undefined)
        charlist = "\s";

    return this.replace(new RegExp("[" + charlist + "]+$"), "");
};

window.onload = function() {
    document.getElementsByClassName("blink_me").fadeIn(3000, blink);
}

const myapp = {
    data() {
        return {
            class_container: "container-xxl",
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
            thumbHistory: [], // store hovered id
            thumbCache: new Map(),
            lastLabel: "", // the last click folder name
            desc: "1",
            clickCounter: 0,
            clickTimer: null,
            search: "",
            isShowing: "",
            history: [],
        }
    },
    async mounted() {
        this.getDf();
        await this.listApi();
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
        var search = _getQS('search');
        if (search != undefined) {
            this.search = decodeURI(search);
            this.listApi();
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
        changeSearch() {
            console.log(this.search);
            _setQS("search", this.search);
            this.listApi();
        },
        async clickDir(path, file) {
            this.hideAllPic();
            var sub = file.Path;
            var p = sub.split("/")
            for (let k in p) {
                if (k > 0) {
                    this.history[k - 1] = p[k] + "/";
                }
            }
            //       console.log(this.history); 
            this.path = sub;
            console.log(this.path);
            console.log(path);
            await this.listApi();
            var nextURL = _host + "/" + this.path;
            console.log(nextURL);
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
            if (this.subListOpen[file.Path] === undefined) {
                this.subListOpen[path, file.Path] = false;
            }
            return this.subListOpen[path, file.Path];
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
        clickSubDir(path, file) {
            console.log(path);
            console.dir(file);
            if (!file.IsDir) {
                return
            }
            var sub = file.Path;
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
        sortTags(tags) {
            if (tags === undefined) {
                return []
            }
            // check if is null
            if (tags === null) {
                return []
            }
            if (tags.length === 0) {
                return []
            }
            return tags.slice().sort((a, b) => a.localeCompare(b));
        },
        async clickUpDir() {
            this.hideAllPic();
            if (this.search.length > 0) {
                this.search = "";
                this.changeSearch();
                return
            }
            this.path = this.resp.UpDir;
            console.log(this.path);
            await this.listApi();
            var nextURL = _host + "/" + this.path;
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
                    this.listApi();
                    this.getDf();
                    _hide();
                })
                .catch(error => {
                    console.log(error)
                })
        },
        easyLoadPic(sub) {
            this.thumbHistory.push("img_" + sub.Hash);
            let thname = document.getElementById("thname").getBoundingClientRect().left;
            let thsize = document.getElementById("thsize").getBoundingClientRect().left;
            let leftPos = thsize - thname;
            let cur = document.getElementById(sub.Hash);
            let rect = cur.getBoundingClientRect();
            let topPos = rect.top;
            console.log(topPos);
            let a = document.createElement("div");
            a.setAttribute("class", "thumb1");
            a.setAttribute("id", "img_" + sub.Hash);
            //a.style.left = leftPos.toString() + "px";
            a.style.left = _left_width;
            let img = document.createElement("img");
            img.src = sub.ShortCut;
            img.setAttribute("class", "thumbimg");
            img.width = window.innerWidth / 2;
            //a.style.top = "-" + topPos.toString() + "px";
            a.style.top = "20px";
            a.appendChild(img);
            cur.appendChild(a);
            console.log(img);
        },
        loadPic(pic, file) {
            let thname = document.getElementById("thname").getBoundingClientRect().left;
            let thsize = document.getElementById("thsize").getBoundingClientRect().left;
            let thlastmod = document.getElementById("thlastmod").getBoundingClientRect().right;
            let leftPos = thsize - thname;

            /*
            _show_width = window.innerWidth - thsize;
            if ( _show_width > 60 ) {
              _show_width = _show_width - 60;
            }
            */

            this.thumbHistory.push("img_" + file.Hash);
            let path = this.path.trimRight("/");
            path = path.trimLeft("/");
            let cur = document.getElementById(file.Hash);
            let rect = cur.getBoundingClientRect();
            let topPos = rect.top;
            let bottomPos = rect.bottom;
            //       console.log(-topPos); 

            let a = document.createElement("div");
            //a.setAttribute("class", "thumb1");
            a.setAttribute("class", "thumb1");
            a.setAttribute("id", "img_" + file.Hash);
            //       a.style.left = leftPos.toString() + "px"; 
            a.style.left = _left_width;
            let img = document.createElement("img");
            if (file.IsImage) {
                img.src = encodeURI('/statics/' + pic.Path);
            } else {
                img.src = pic.Path + '?max-width=' + window.innerWidth / 2;
            }
            console.log(file.ShortCut);
            console.log(img.src);
            img.setAttribute("class", "thumbimg");

            //       let imgPath = encodeURI(_host + '/photo/' + path + "/" + file.Name); 
            let imgPath = ""
            if (file.IsImage) {
                imgPath = encodeURI(_host + '/statics/' + file.Path);
            } else {
                imgPath = encodeURI(_host + '/photo/' + file.Path);
            }
            //       console.log(imgPath); 

            a.onclick = function() {
                window.open(
                    imgPath,
                    '_blank' // <- This is what makes it open in a new window.
                );
            }
            if (file.IsImage) {
                //img.width = _show_width;
                //a.style.top = "-" + topPos.toString() + "px";
                a.style.top = "20px";
                a.appendChild(img);
                cur.appendChild(a);
                return
            }


            if (pic.Width > window.innerWidth / 2) {
                img.width = window.innerWidth / 2;
            } else {
                img.width = pic.Width;
            }
            let zoom = img.width / pic.Width; // like 0.5
            img.height = pic.Height * zoom;

            /*
            if (img.height > screen.height) {
              console.log("case 1 " + pic.Height);
              if (img.height > topPos) {
                a.style.top = "-" + topPos.toString() + "px"; // from top of screen
              } else {
                let x = bottomPos - img.height - topPos;
                a.style.top = x.toString() + "px";
              }
            } else if (img.height > topPos) {
              console.log("case 2");
              a.style.top = "-" + topPos.toString() + "px";
            } else {
              console.log("case 3");
              let x = bottomPos - img.height - topPos;
              a.style.top = x.toString() + "px";
            }
            if (file.IsImage) {
              a.style.top = "0px";
              img.height = "auto";
              img.width = "auto";
            }
            */
            a.style.top = "20px";

            a.appendChild(img);
            cur.appendChild(a);
        },
        autoPic(file) {
            if (this.isShowing == file.Path) {
                this.hidePic(file);
                this.isShowing = "";
            } else {
                this.showPic(file);
                this.isShowing = file.Path;
            }
        },
        async singlePic(sub) {
            this.hideAllPic();
            console.log(sub);
            if (!sub.IsImage) {
                return
            }
            console.log(sub.ShortCut);
            this.easyLoadPic(sub);
        },
        async showPic(file) {
            this.hideAllPic();
            if (file.IsImage) {
                this.loadPic(file, file);
                return
            }
            if (!file.IsDir) {
                return
            }
            //       console.log(this.thumbHistory.length); 
            var data = {};
            data.path = file.Path;
            if (this.thumbCache.has(file.Hash)) {
                //         console.log("HIT " + file.Hash); 
                this.loadPic(this.thumbCache.get(file.Hash), file);
                return
            }
            await axios.post("/api?action=thumb", data)
                .then(response => {
                    if (response.data.Data !== "") {
                        let pic = response.data.Data;
                        this.thumbCache.set(file.Hash, pic);
                        //             console.log("MISS " + file.Hash); 
                        this.loadPic(pic, file);
                    }
                })
                .catch(error => {
                    console.log(error)
                })

            //let img = document.getElementById('img_' + id);
            //img.style.display = 'inline';
        },
        hidePic(file) {
            if (!file.IsDir) {
                return
            }
            let div = document.getElementById('img_' + file.Hash);
            if (div !== null) {
                div.remove();
                //         this.thumbHistory.pop(); 
            }
        },
        hideAllPic() {
            if (this.thumbHistory.length > 0) {
                for (let i = 0; i < this.thumbHistory.length; i++) {
                    let div = document.getElementById(this.thumbHistory[i]);
                    if (div !== null) {
                        div.remove();
                    }
                }
                this.thumbHistory = [];
            }
        },
        async listApi() {
            var data = {};
            data.path = this.path;
            data.list = "read";
            data.search = this.search;
            await axios.post("/api?action=list", data)
                .then(response => {
                    this.resp = response.data.Data;
                    this.updir = this.resp.UpDir;
                    this.dir = this.resp.Dir;
                    this.files = this.resp.Files;
                    for (let i = 0; i < this.files.length; i++) {
                        this.labelMap[this.files[i].Name] = this.files[i].Meta.Label;
                        //             this.files[i].Tags.sort(); 
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
                    this.listApi();
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
