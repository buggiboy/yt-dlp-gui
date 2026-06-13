export namespace main {
	
	export class DepsStatus {
	    python: boolean;
	    ytdlp: boolean;
	    extras: boolean;
	    ffmpeg: boolean;
	    ffmpegSkipped: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DepsStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.python = source["python"];
	        this.ytdlp = source["ytdlp"];
	        this.extras = source["extras"];
	        this.ffmpeg = source["ffmpeg"];
	        this.ffmpegSkipped = source["ffmpegSkipped"];
	    }
	}
	export class DownloadOptions {
	    url: string;
	    start: string;
	    end: string;
	    quality: string;
	    audioFormat: string;
	    folder: string;
	    subtitles: boolean;
	    subLangs: string;
	    embedMeta: boolean;
	    sponsorBlock: string[];
	    rateLimit: string;
	    concurrentFragments: number;
	    extraArgs: string;
	    outtmpl: string;
	    nameArgs: string[];
	
	    static createFrom(source: any = {}) {
	        return new DownloadOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.quality = source["quality"];
	        this.audioFormat = source["audioFormat"];
	        this.folder = source["folder"];
	        this.subtitles = source["subtitles"];
	        this.subLangs = source["subLangs"];
	        this.embedMeta = source["embedMeta"];
	        this.sponsorBlock = source["sponsorBlock"];
	        this.rateLimit = source["rateLimit"];
	        this.concurrentFragments = source["concurrentFragments"];
	        this.extraArgs = source["extraArgs"];
	        this.outtmpl = source["outtmpl"];
	        this.nameArgs = source["nameArgs"];
	    }
	}
	export class VideoFormat {
	    height: number;
	    ext: string;
	
	    static createFrom(source: any = {}) {
	        return new VideoFormat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.height = source["height"];
	        this.ext = source["ext"];
	    }
	}
	export class FormatList {
	    videos: VideoFormat[];
	    audioExt: string;
	
	    static createFrom(source: any = {}) {
	        return new FormatList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.videos = this.convertValues(source["videos"], VideoFormat);
	        this.audioExt = source["audioExt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PreviewInfo {
	    kind: string;
	    url: string;
	    thumbnail: string;
	    title: string;
	    duration: string;
	
	    static createFrom(source: any = {}) {
	        return new PreviewInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.url = source["url"];
	        this.thumbnail = source["thumbnail"];
	        this.title = source["title"];
	        this.duration = source["duration"];
	    }
	}

}

