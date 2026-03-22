import { useSearchParams } from "react-router-dom";

export default function Login() {
  const [searchParams] = useSearchParams();
  const error = searchParams.get("error");

  return (
    <div className="page">
      <h1>Vibe Seeker</h1>
      <p>Discover venues that match your vibe.</p>
      {error && <p className="error">Login failed: {error}</p>}
      <a href="/api/auth/login" className="button">
        Log in with Spotify
      </a>
    </div>
  );
}
