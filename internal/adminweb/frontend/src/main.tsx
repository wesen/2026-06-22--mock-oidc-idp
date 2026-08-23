import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Provider } from "react-redux";
import "bootstrap/dist/css/bootstrap.min.css";
import "@go-go-golems/rag-evaluation-site/styles.css";
import "@go-go-golems/rag-evaluation-site/theme.css";
import "./styles.css";
import { App } from "./App";
import { store } from "./store";

const root = document.getElementById("root");
if (!root) {
  throw new Error("TinyIDP Console root element is missing");
}

createRoot(root).render(
  <StrictMode>
    <Provider store={store}>
      <App />
    </Provider>
  </StrictMode>,
);
