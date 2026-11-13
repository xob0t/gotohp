import { Clipboard, Events } from "@wailsio/runtime";
import { ConfigManager } from "../../bindings/app/core";
import { reactive } from "vue";
import { debugLog } from "./debug";
import {
  recordUploadResult,
  type UploadResults,
} from "./uploadResults";

export interface ThreadStatus {
  WorkerID: number;
  Status: string;
  FilePath: string;
  FileName: string;
  Message: string;
  BytesUploaded: number;
  BytesTotal: number;
  Attempt: number;
}

export interface FileUploadResult {
  MediaKey: string;
  IsError: boolean;
  IsLivePhoto: boolean;
  Skipped: boolean;
  ErrorMessage: string;
  SkipCode: string;
  SkipReason: string;
  Path: string;
  Paths: string[];
}

export interface PreflightWarning {
  Paths: string[];
  Code: string;
  Message: string;
}

export interface UploadBatchStart {
  Total: number;
  TotalBytes: number;
}

export interface AlbumStatus {
  AlbumName: string;
  ItemsAdded: number;
  TotalItems: number;
  AlbumKeys: string[];
  IsComplete: boolean;
}

export interface AlbumError {
  AlbumName: string;
  Error: string;
}

function isSkippedUploadWarning(code: string): boolean {
  return code === "incomplete-live-photo-skipped" || code === "ambiguous-filename-stem";
}

/** One drop: the dropped paths plus the album choice made for them. */
export interface UploadJob {
  files: string[];
  albumName: string;
  albumAutoMode: boolean;
}

// The backend clears its "running" flag just after it reports uploadStop, so the
// next queued job is started after a short pause and re-sent if it was ignored.
const NEXT_JOB_DELAY_MS = 400;
const START_RETRY_MS = 1000;
const MAX_START_ATTEMPTS = 5;

/** A finished file, newest first in UploadState.recent. */
export interface RecentItem {
  path: string;
  status: "success" | "skipped" | "fail";
}

const MAX_RECENT_ITEMS = 60;

export interface UploadState {
  // The latest finished files of this run, for the gallery on wide windows
  recent: RecentItem[];
  isUploading: boolean;
  // True while a queued job has been handed to the backend but has not started yet
  isStarting: boolean;
  // Drops waiting for the current upload to finish
  queue: UploadJob[];
  // Totals for the whole run: the first job plus everything queued behind it
  runStartTime: number;
  runBytes: number;
  runJobs: number;
  totalFiles: number;
  uploadedFiles: number;
  threads: Map<number, ThreadStatus>;
  results: UploadResults;
  warnings: PreflightWarning[];
  // Byte tracking
  totalBytes: number;
  uploadedBytes: number;
  // Timing
  startTime: number;
  // Set when the batch stops; 0 while a batch is running
  endTime: number;
  // Speed calculation (bytes per second)
  uploadSpeed: number;
  peakSpeed: number;
  // Album creation
  albumStatus: AlbumStatus | null;
  isCreatingAlbum: boolean;
}

class UploadManager {
  private static instance: UploadManager;

  // Reactive state that can be accessed by components
  public state = reactive<UploadState>({
    recent: [],
    isUploading: false,
    isStarting: false,
    queue: [],
    runStartTime: 0,
    runBytes: 0,
    runJobs: 0,
    totalFiles: 0,
    uploadedFiles: 0,
    threads: new Map<number, ThreadStatus>(),
    results: {
      success: [],
      fail: [],
      skipped: [],
      warnings: [],
    },
    warnings: [],
    totalBytes: 0,
    uploadedBytes: 0,
    startTime: 0,
    endTime: 0,
    uploadSpeed: 0,
    peakSpeed: 0,
    albumStatus: null,
    isCreatingAlbum: false,
  });

  // For speed calculation
  private lastSpeedUpdate: number = 0;
  private lastBytesUploaded: number = 0;
  private speedSamples: number[] = [];
  // Track bytes from completed files
  private completedBytes: number = 0;
  // Track the last known BytesTotal for each file
  private fileBytes: Map<string, number> = new Map();
  // Dedupe decisions can change the bytes that will actually be uploaded.
  private totalBytesAdjustment: number = 0;

  // Queue handling
  private currentJob: UploadJob | null = null;
  private awaitingStart = false;
  private startAttempts = 0;
  private startTimer: ReturnType<typeof setTimeout> | null = null;
  private advanceTimer: ReturnType<typeof setTimeout> | null = null;
  // True for jobs started from the queue; their results add to the run's results
  private continuingRun = false;

  private constructor() {
    // Bind all methods to ensure 'this' context is preserved
    this.resetUploadResults = this.resetUploadResults.bind(this);
    this.cancelUpload = this.cancelUpload.bind(this);
    this.copyResultsAsJson = this.copyResultsAsJson.bind(this);
    this.enqueue = this.enqueue.bind(this);
    this.clearQueue = this.clearQueue.bind(this);

    this.setupEventListeners();
  }

  public static getInstance(): UploadManager {
    if (!UploadManager.instance) {
      UploadManager.instance = new UploadManager();
    }
    return UploadManager.instance;
  }

  private setupEventListeners() {
    // Handle upload start
    Events.On("uploadStart", (event: { data: UploadBatchStart }) => {
      debugLog("event", "backend", "uploadStart", event.data);
      this.state.totalFiles = event.data.Total;
      this.state.totalBytes = event.data.TotalBytes; // May be 0 initially
      this.state.uploadedFiles = 0;
      this.state.uploadedBytes = 0;
      this.state.isUploading = true;
      this.state.isStarting = false;
      this.awaitingStart = false;
      this.clearStartTimer();
      this.state.threads.clear();
      this.state.startTime = Date.now();
      this.state.endTime = 0;
      this.state.uploadSpeed = 0;
      this.state.peakSpeed = 0;
      this.state.albumStatus = null;
      this.state.isCreatingAlbum = false;
      this.lastSpeedUpdate = Date.now();
      this.lastBytesUploaded = 0;
      this.speedSamples = [];
      this.completedBytes = 0;
      this.fileBytes.clear();
      this.totalBytesAdjustment = 0;
      if (!this.continuingRun) {
        this.resetUploadResults();
      }
      this.state.warnings = [];
    });

    // Handle async total bytes update (calculated after uploadStart)
    Events.On("uploadTotalBytes", (event: { data: number }) => {
      debugLog("event", "backend", "uploadTotalBytes", event.data);
      this.state.totalBytes = Math.max(0, event.data + this.totalBytesAdjustment);
    });

    Events.On("uploadTotalBytesDelta", (event: { data: number }) => {
      debugLog("event", "backend", "uploadTotalBytesDelta", event.data);
      this.totalBytesAdjustment += event.data;
      this.state.totalBytes = Math.max(0, this.state.totalBytes + event.data);
    });

    Events.On("uploadWarning", (event: { data: PreflightWarning }) => {
      debugLog("warn", "backend", "uploadWarning", event.data);
      this.state.warnings.push(event.data);
      if (!isSkippedUploadWarning(event.data.Code)) {
        this.state.results.warnings.push({
          paths: event.data.Paths,
          code: event.data.Code,
          reason: event.data.Message,
        });
      }
    });

    // Handle thread status updates
    Events.On("ThreadStatus", (event: { data: ThreadStatus }) => {
      const thread = event.data;
      debugLog(thread.Status === "error" ? "error" : "event", "backend", "ThreadStatus", thread);
      const prevThread = this.state.threads.get(thread.WorkerID);

      if (thread.Status === 'uploading' && thread.FilePath && thread.BytesTotal > 0) {
        this.fileBytes.set(thread.FilePath, thread.BytesTotal);
      }

      if (thread.Status === 'error' && prevThread?.Message !== thread.Message) {
        window.dispatchEvent(new CustomEvent('uploadError', { detail: thread }));
      }
      
      this.state.threads.set(thread.WorkerID, thread);
      this.updateBytesAndSpeed();
    });

    // Handle file status updates
    Events.On("FileStatus", (event: { data: FileUploadResult }) => {
      const { Path, Paths } = event.data;
      debugLog(event.data.IsError ? "error" : "event", "backend", "FileStatus", event.data);
      if (Path) {
        this.state.recent.unshift({
          path: Path,
          status: event.data.IsError ? "fail" : event.data.Skipped ? "skipped" : "success",
        });
        if (this.state.recent.length > MAX_RECENT_ITEMS) {
          this.state.recent.length = MAX_RECENT_ITEMS;
        }
      }

      this.state.uploadedFiles += recordUploadResult(this.state.results, event.data);
      if (!event.data.IsError) {
        const completedFileBytes = this.fileBytes.get(Path);
        if (completedFileBytes && completedFileBytes > 0) {
          this.completedBytes += completedFileBytes;
        }
      }
      for (const completedPath of Paths?.length ? Paths : [Path]) {
        this.fileBytes.delete(completedPath);
      }
      this.updateBytesAndSpeed();

      // Skipped-only batches can begin and end within one backend event burst.
      // Finishing locally prevents a missed terminal event from leaving the timer running.
      if (
        this.state.totalFiles > 0
        && this.state.uploadedFiles >= this.state.totalFiles
        && this.state.results.success.length === 0
        && this.state.results.fail.length === 0
      ) {
        this.finishBatch();
      }
    });

    // Handle upload stop
    Events.On("uploadStop", () => {
      debugLog("event", "backend", "uploadStop");
      this.finishBatch();
    });

    // Handle album creation progress
    Events.On("albumProgress", (event: { data: AlbumStatus }) => {
      debugLog("event", "backend", "albumProgress", event.data);
      this.state.albumStatus = event.data;
      this.state.isCreatingAlbum = true;
    });

    // Handle album creation complete
    Events.On("albumComplete", (event: { data: AlbumStatus }) => {
      debugLog("event", "backend", "albumComplete", event.data);
      this.state.albumStatus = event.data;
      this.state.isCreatingAlbum = false;
    });

    // Handle album creation error
    Events.On("albumError", (event: { data: AlbumError }) => {
      debugLog("error", "backend", "albumError", event.data);
      this.state.isCreatingAlbum = false;
      // Emit a custom event that App.vue can listen to for showing toast
      window.dispatchEvent(new CustomEvent('albumError', { detail: event.data }));
    });
  }

  private finishBatch() {
    if (!this.state.isUploading) {
      // Already finished locally; a late uploadStop must not start the next job twice.
      return;
    }
    if (this.state.endTime === 0) {
      this.state.endTime = Date.now();
    }
    this.state.runBytes += this.state.uploadedBytes;
    this.state.isUploading = false;
    this.scheduleNextJob();
  }

  /** True from the moment a drop is accepted until the last queued job has finished. */
  public isBusy(): boolean {
    return this.state.isUploading || this.state.isStarting || this.awaitingStart;
  }

  /**
   * Uploads a drop now, or queues it behind the running upload. Rejects only when
   * an immediate start fails; queued jobs report their failures as upload errors.
   */
  public async enqueue(job: UploadJob): Promise<"started" | "queued"> {
    if (this.isBusy()) {
      this.state.queue.push(job);
      debugLog("info", "frontend", "upload queued", {
        fileCount: job.files.length,
        albumName: job.albumName,
        albumAutoMode: job.albumAutoMode,
        queueLength: this.state.queue.length,
      });
      return "queued";
    }

    this.continuingRun = false;
    this.state.runStartTime = Date.now();
    this.state.runBytes = 0;
    this.state.runJobs = 0;
    await this.startJob(job);
    return "started";
  }

  public clearQueue() {
    if (this.state.queue.length > 0) {
      debugLog("info", "frontend", "upload queue cleared", { dropped: this.state.queue.length });
    }
    this.state.queue = [];
  }

  private async startJob(job: UploadJob) {
    this.currentJob = job;
    this.awaitingStart = true;
    this.startAttempts = 0;
    try {
      // The backend reads the album choice when the upload starts (not persisted to disk)
      await ConfigManager.SetAlbumName(job.albumName);
      await ConfigManager.SetAlbumAutoMode(job.albumAutoMode);
    } catch (error) {
      this.awaitingStart = false;
      this.currentJob = null;
      throw error;
    }
    this.state.runJobs += 1;
    this.sendStart();
  }

  private sendStart() {
    const job = this.currentJob;
    if (!job) return;

    this.startAttempts += 1;
    debugLog("info", "frontend", "startUpload", {
      fileCount: job.files.length,
      albumName: job.albumName,
      albumAutoMode: job.albumAutoMode,
      attempt: this.startAttempts,
    });
    Events.Emit("startUpload", { files: job.files });

    this.clearStartTimer();
    this.startTimer = setTimeout(() => {
      this.startTimer = null;
      if (!this.awaitingStart) return;
      if (this.startAttempts < MAX_START_ATTEMPTS) {
        this.sendStart();
        return;
      }
      this.awaitingStart = false;
      this.state.isStarting = false;
      this.reportJobFailure("The upload did not start.");
      this.scheduleNextJob();
    }, START_RETRY_MS);
  }

  private scheduleNextJob() {
    if (this.advanceTimer !== null || this.state.queue.length === 0) return;

    this.state.isStarting = true;
    this.advanceTimer = setTimeout(() => {
      this.advanceTimer = null;
      const next = this.state.queue.shift();
      if (!next) {
        this.state.isStarting = false;
        return;
      }
      this.continuingRun = true;
      this.startJob(next).catch((error) => {
        this.state.isStarting = false;
        this.reportJobFailure(error instanceof Error ? error.message : String(error));
        this.scheduleNextJob();
      });
    }, NEXT_JOB_DELAY_MS);
  }

  private reportJobFailure(message: string) {
    debugLog("error", "frontend", "queued upload failed to start", message);
    window.dispatchEvent(new CustomEvent("uploadError", { detail: { FileName: "", Message: message } }));
  }

  private clearStartTimer() {
    if (this.startTimer !== null) {
      clearTimeout(this.startTimer);
      this.startTimer = null;
    }
  }

  /** Snapshot of the private byte accounting, shown in the debug panel. */
  public getInternals() {
    return {
      completedBytes: this.completedBytes,
      totalBytesAdjustment: this.totalBytesAdjustment,
      trackedFiles: this.fileBytes.size,
      awaitingStart: this.awaitingStart,
      startAttempts: this.startAttempts,
      continuingRun: this.continuingRun,
      speedSamples: this.speedSamples.map((sample) => Math.round(sample)),
      lastSpeedUpdate: this.lastSpeedUpdate,
    };
  }

  private updateBytesAndSpeed() {
    // Calculate bytes from currently active uploads
    let activeUploadedBytes = 0;

    this.state.threads.forEach((thread) => {
      if (thread.Status === 'uploading' && thread.BytesTotal > 0) {
        activeUploadedBytes += thread.BytesUploaded;
      } else if (thread.Status === 'finalizing' && thread.FilePath) {
        activeUploadedBytes += this.fileBytes.get(thread.FilePath) ?? 0;
      }
    });

    // Total uploaded = completed files + current progress
    const totalUploaded = this.completedBytes + activeUploadedBytes;
    this.state.uploadedBytes = totalUploaded;

    // Calculate speed (using rolling average)
    const now = Date.now();
    const timeDelta = now - this.lastSpeedUpdate;

    if (timeDelta >= 500) { // Update speed every 500ms
      const bytesDelta = totalUploaded - this.lastBytesUploaded;
      const instantSpeed = (bytesDelta / timeDelta) * 1000; // bytes per second

      if (instantSpeed >= 0) {
        this.speedSamples.push(instantSpeed);
        // Keep last 5 samples for smoothing
        if (this.speedSamples.length > 5) {
          this.speedSamples.shift();
        }
        // Calculate average speed
        this.state.uploadSpeed = this.speedSamples.reduce((a, b) => a + b, 0) / this.speedSamples.length;
        this.state.peakSpeed = Math.max(this.state.peakSpeed, this.state.uploadSpeed);
      }

      this.lastSpeedUpdate = now;
      this.lastBytesUploaded = totalUploaded;
    }
  }

  public resetUploadResults() {
    this.state.results.success = [];
    this.state.results.fail = [];
    this.state.results.skipped = [];
    this.state.results.warnings = [];
    this.state.recent = [];
  }

  /** Cancels the running upload and drops everything queued behind it. */
  public cancelUpload() {
    debugLog("info", "frontend", "uploadCancel");
    this.clearQueue();
    if (this.advanceTimer !== null) {
      clearTimeout(this.advanceTimer);
      this.advanceTimer = null;
    }
    this.state.isStarting = false;
    Events.Emit("uploadCancel");
  }

  public async copyResultsAsJson() {
    const resultsJson = JSON.stringify(this.state.results, null, 2);
    try {
      await Clipboard.SetText(resultsJson);
      console.log("Upload results copied to clipboard");
      return true;
    } catch (error) {
      console.error("Failed to copy results:", error);
      return false;
    }
  }
}

// Create and export a single instance
export const uploadManager = UploadManager.getInstance();
