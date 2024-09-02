import { BrowserRouter as Router, Routes, Route } from "react-router-dom";
import "./output.css";
import Home from "./pages/home";
import Results from "./pages/results";

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/results" element={<Results />} />
      </Routes>
    </Router>
  );
}

export default App;