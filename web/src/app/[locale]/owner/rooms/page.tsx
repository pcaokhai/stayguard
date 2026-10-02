import { Suspense } from "react";
import { RoomMap } from "@/features/rooms/RoomMap";

export default function OwnerRoomsPage() {
  return (
    <Suspense>
      <RoomMap owner />
    </Suspense>
  );
}
