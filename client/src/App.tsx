import { BrowserRouter as Router, Routes, Route, useLocation } from "react-router-dom";
import { trackPageView } from "./analytics";
import { useEffect } from "react";
import "./output.css";
import Home from "./pages/home";
import Results from "./pages/results";

function App() {

  const location = useLocation();

  useEffect(() => {
    trackPageView(location.pathname + location.search);
  }, [location]);


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