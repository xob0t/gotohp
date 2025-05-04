// src/components/wails-controls/utils/window.ts
import { ref } from "vue";
import { Window } from "@wailsio/runtime";

export const isWindowMaximized = ref(false);

// Initialize window state
Window.IsMaximised().then((maximized) => {
  isWindowMaximized.value = maximized;
});

// // Listen for window state changes
// Window.OnResize(() => {
//   Window.IsMaximised().then((maximized) => {
//     isWindowMaximized.value = maximized
//   })
// })

export const minimizeWindow = async () => {
  await Window.Minimise();
};

export const maximizeWindow = async () => {
  if (isWindowMaximized.value) {
    await Window.UnMaximise();
  } else {
    await Window.Maximise();
  }
  isWindowMaximized.value = !isWindowMaximized.value;
};

export const fullscreenWindow = async () => {
  const isFullscreen = await Window.IsFullscreen();
  if (isFullscreen) {
    await Window.UnFullscreen();
  } else {
    await Window.Fullscreen();
  }
};

export const closeWindow = async () => {
  await Window.Close();
};
