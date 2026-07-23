import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { MotionConfig } from "framer-motion";
import { ReleasesPage } from "./pages/ReleasesPage";
import "./index.css";

const root = document.getElementById("root");
if (!root) throw new Error("Root element #root not found");

createRoot(root).render(
  <StrictMode>
    <MotionConfig reducedMotion="user">
      <ReleasesPage />
    </MotionConfig>
  </StrictMode>,
);
