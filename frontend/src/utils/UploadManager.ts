import { Clipboard, Events } from "@wailsio/runtime";
import { reactive } from "vue";

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
  Path: string;
}

export interface UploadBatchStart {
  Total: number;
  TotalBytes: number;
}

export interface UploadSuccess {
  path: string;
  mediaKey: string;
}

export interface UploadState {
  isUploading: boolean;
  totalFiles: number;
  uploadedFiles: number;
  threads: Map<number, ThreadStatus>;
  results: {
    success: UploadSuccess[];
    fail: string[];
  };
  // Byte tracking
  totalBytes: number;
  uploadedBytes: number;
  // Timing
  startTime: number;
  // Speed calculation (bytes per second)
  uploadSpeed: number;
}

class UploadManager {
  private static instance: UploadManager;

  // Reactive state that can be accessed by components
  public state = reactive<UploadState>({
    isUploading: false,
    totalFiles: 0,
    uploadedFiles: 0,
    threads: new Map<number, ThreadStatus>(),
    results: {
      success: [],
      fail: [],
    },
    totalBytes: 0,
    uploadedBytes: 0,
    startTime: 0,
    uploadSpeed: 0,
  });

  // For speed calculation
  private lastSpeedUpdate: number = 0;
  private lastBytesUploaded: number = 0;
  private speedSamples: number[] = [];
  // Track bytes from completed files
  private completedBytes: number = 0;
  // Track the last known BytesTotal for each worker to detect file completion
  private lastThreadBytes: Map<number, number> = new Map();

  private constructor() {
    // Bind all methods to ensure 'this' context is preserved
    this.resetUploadResults = this.resetUploadResults.bind(this);
    this.cancelUpload = this.cancelUpload.bind(this);
    this.copyResultsAsJson = this.copyResultsAsJson.bind(this);

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
      this.state.totalFiles = event.data.Total;
      this.state.totalBytes = event.data.TotalBytes;
      this.state.uploadedFiles = 0;
      this.state.uploadedBytes = 0;
      this.state.isUploading = true;
      this.state.threads.clear();
      this.state.startTime = Date.now();
      this.state.uploadSpeed = 0;
      this.lastSpeedUpdate = Date.now();
      this.lastBytesUploaded = 0;
      this.speedSamples = [];
      this.completedBytes = 0;
      this.lastThreadBytes.clear();
      this.resetUploadResults();
    });

    // Handle thread status updates
    Events.On("ThreadStatus", (event: { data: ThreadStatus }) => {
      const thread = event.data;
      const prevThread = this.state.threads.get(thread.WorkerID);
      
      // Detect when a file upload completes (status changes from uploading to completed/idle/error)
      if (prevThread && prevThread.Status === 'uploading' && thread.Status !== 'uploading') {
        // Add the completed file's total bytes to completedBytes
        if (prevThread.BytesTotal > 0) {
          this.completedBytes += prevThread.BytesTotal;
        }
      }
      
      this.state.threads.set(thread.WorkerID, thread);
      this.updateBytesAndSpeed();
    });

    // Handle file status updates
    Events.On("FileStatus", (event: { data: { IsError: boolean; Path: string; MediaKey: string } }) => {
      const { IsError, Path, MediaKey } = event.data;

      if (!IsError) {
        this.state.uploadedFiles += 1;
        this.state.results.success.push({ path: Path, mediaKey: MediaKey });
      } else {
        this.state.results.fail.push(Path);
      }
    });

    // Handle upload stop
    Events.On("uploadStop", () => {
      this.state.isUploading = false;
    });
  }

  private updateBytesAndSpeed() {
    // Calculate bytes from currently active uploads
    let activeUploadedBytes = 0;

    this.state.threads.forEach((thread) => {
      if (thread.Status === 'uploading' && thread.BytesTotal > 0) {
        activeUploadedBytes += thread.BytesUploaded;
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
      }

      this.lastSpeedUpdate = now;
      this.lastBytesUploaded = totalUploaded;
    }
  }

  public resetUploadResults() {
    this.state.results.success = [];
    this.state.results.fail = [];
  }

  public cancelUpload() {
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
