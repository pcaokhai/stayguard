import en from "../../messages/en.json";

// Placeholder until SG-004 adds locale routes; English only for now.
export default function Page() {
  return (
    <main>
      <h1>{en.app.title}</h1>
      <p>{en.app.tagline}</p>
    </main>
  );
}
