import { useSearchParams } from "react-router-dom";

const API_URL = import.meta.env.VITE_API_URL || "http://127.0.0.1:8080";

export default function Login() {
  const [searchParams] = useSearchParams();
  const error = searchParams.get("error");

  return (
    <div className="page">
      <h1>Vibe Seeker</h1>
      <p>Discover venues that match your music taste.</p>
      {error && <p className="error">Login failed: {error}</p>}
      <a href={`${API_URL}/api/auth/login`} className="button">
        Log in with Spotify
      </a>
    </div>
  );
}
