import { reactive } from "vue";
import { Events, Clipboard } from "@wailsio/runtime";

export interface UploadSuccess {
  path: string;
  mediaKey: string;
}

export interface UploadState {
  isUploading: boolean;
  totalFiles: number;
  uploadedFiles: number;
  results: {
    success: UploadSuccess[];
    fail: string[];
  };
}

class UploadManager {
  private static instance: UploadManager;

  // Reactive state that can be accessed by components
  public state = reactive<UploadState>({
    isUploading: false,
    totalFiles: 0,
    uploadedFiles: 0,
    results: {
      success: [],
      fail: [],
    },
  });

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
    Events.On("uploadStart", (event: { data: Array<{ Total: number }> }) => {
      this.state.totalFiles = event.data[0].Total;
      this.state.uploadedFiles = 0;
      this.state.isUploading = true;
      this.resetUploadResults();
    });

    // Handle file status updates
    Events.On("FileStatus", (event: { data: Array<{ IsError: boolean; Path: string; MediaKey: string }> }) => {
      const { IsError, Path, MediaKey } = event.data[0];

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
