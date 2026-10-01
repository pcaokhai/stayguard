import { redirect } from "next/navigation";

// Only /vi is generated; the bare root forwards there.
export default function Page() {
  redirect("/vi");
}
