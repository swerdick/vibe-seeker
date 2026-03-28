import { BrowserRouter, Routes, Route } from "react-router-dom";
import { AuthProvider } from "./contexts/AuthContext";
import ProtectedRoute from "./components/ProtectedRoute";
import Login from "./pages/Login";
import Callback from "./pages/Callback";
import Home from "./pages/Home";
import Explore from "./pages/Explore";
import "./App.css";

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<Login />} />
          <Route path="/callback" element={<Callback />} />
          <Route element={<ProtectedRoute />}>
            <Route path="/home" element={<Home />} />
          </Route>
          <Route path="/explore" element={<Explore />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;
