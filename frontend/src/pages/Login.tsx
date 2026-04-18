import { useSearchParams, Link } from "react-router-dom";

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
      <p className="login-disclaimer">
        Spotify login is currently limited to approved accounts.
        If you have to ask, you're probably not on the list — sorry!
      </p>
      <Link to="/explore" className="button button-secondary" style={{ marginTop: "0.75rem" }}>
        Explore without login
      </Link>
    </div>
  );
}
