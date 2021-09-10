window.imgscroll = {
	options: {
		target: null, //插入图片的目标位置
		img_list: null, //图片数组 [{ url: "/CMF01_000.jpg"},{ url: "/CMF01_001.jpg"}]
		img_max: 0, //图片数量
		img_num: 0, //图片累计已加载的数量
		step_max: 50, //每轮加载图片的数量 从0开始计数
		step_num: 0, //每轮已加载图片的数量
		img_obj: new Image(),
		s_scroll: 0, //滑动条的Y轴位置
		w_height: 0, //页面内容的高度
		l_height: 1200, //小于此参数则开始加载图片
		w_width: 640 //浏览器窗口宽度
	},
	onLoad: function(){
		if(this.options.img_num >= this.options.img_max){
			$("#img_load").hide(); //隐藏loading图标
			$("#control_block").show();
			return;
		}
		this.options.img_obj.src = this.options.img_list[this.options.img_num].url;
		this.options.img_obj.onload = function(){
			imgscroll.endLoad(this.width);
		};
	},
	endLoad: function(width){
		for ( var i = 0; i < 5; i++ ){
			if(this.options.img_num >= this.options.img_max){
				$("#img_load").hide(); //隐藏loading图标
				$("#control_block").show();
				return;
			}
			width = this.options.w_width > width? width+"px": "99%";
			this.options.target.append('<div style="text-align:center;color:#999;padding-bottom:10px;font-size:13px;"><img src="'+this.options.img_list[this.options.img_num].url+'" width="'+width+'"><br /><span>'+ (this.options.img_num+1) +'/'+ this.options.img_max +'</span></div>');
			this.options.img_num += 1;
		}
		if(this.options.step_num < this.options.step_max){
			this.options.step_num += 1;
			this.onLoad();
		}else{
			//结束一轮加载后将每轮已加载图片数量归零
			this.options.step_num = 0;
		}
	},
	//target:目标元素 imglist:图片数组 benum:图片开始加载的位置
	beLoad: function(target,img_list,benum){
		this.options.target = target;
		this.options.img_list = img_list;
		this.options.img_max = img_list.length;
		this.options.img_num = benum;
		this.options.l_height = $(window).height()*2;
		this.options.w_width = $("body").width();
		//绑定滑动条的判定
		$(window).scroll(function(){
//			if(window.citeDis) return;
//			imgscroll.options.s_scroll = $(window).scrollTop();
//			imgscroll.options.w_height = $("body").height();
//			if((imgscroll.options.w_height-imgscroll.options.s_scroll) < imgscroll.options.l_height){
//				if(imgscroll.options.step_num < 1) imgscroll.onLoad();
//			}
			imgscroll.onLoad();
		});
		this.onLoad();
	}

};

//是否允许缩放
function changeMeta(isScl){
	var meta = document.getElementsByTagName('meta');
	if(isScl){
		meta[0].setAttribute('content',"width=device-width, initial-scale=1.0, maximum-scale=3.0, user-scalable=3.0;");
	}else{
		meta[0].setAttribute('content',"width=device-width, initial-scale=1.0, user-scalable=no");
	}
}

window.autoScroll = {
	wsTop: 0,
	wStep: 0,
	wNum: 0,
	wPx: 5,
	wUp: 0,
	setTime: null,
	autoSrl: function(){
		if(autoScroll.wUp < 1){
			$(window).scrollTop(autoScroll.wsTop + autoScroll.wPx*autoScroll.wStep);
		}else{
			$(window).scrollTop(autoScroll.wsTop - autoScroll.wPx*autoScroll.wStep);
		}
		if(autoScroll.wStep < autoScroll.wNum){
			autoScroll.wStep += 1;
			autoScroll.setTime = setTimeout(autoScroll.autoSrl,5);
		}else{
			autoScroll.wStep = 0;
			autoScroll.setTime = null;
		}
	},
	beAuto: function(up){
		if(autoScroll.setTime == null){
			if(up < 1){
				autoScroll.wUp = 0;
			}else{
				autoScroll.wUp = 1;
			}
			this.autoSrl();
		}
	}
}
