import { useNavigate } from "react-router-dom";

export default function Home() {
  const navigate = useNavigate();
  const token = localStorage.getItem("token");

  if (!token) {
    navigate("/", { replace: true });
    return null;
  }

  const handleLogout = () => {
    localStorage.removeItem("token");
    navigate("/", { replace: true });
  };

  return (
    <div className="page">
      <h1>Hello World</h1>
      <p>You are logged in with Spotify.</p>
      <button onClick={handleLogout}>Log out</button>
    </div>
  );
}
