import { Suspense } from "react";
import { RoomMap } from "../../../features/rooms/RoomMap";

export default function RoomsPage() {
  return (
    <Suspense>
      <RoomMap />
    </Suspense>
  );
}
