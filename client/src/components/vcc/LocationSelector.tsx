
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { MapPin } from "lucide-react";
import { type CameraOption } from "./CameraSelector";

interface LocationSelectorProps {
    locations: string[];
    selectedLocation: string | null;
    onSelect: (location: string | null) => void;
    className?: string; // Allow passing custom classes
}

export function LocationSelector({
    locations,
    selectedLocation,
    onSelect,
    className
}: LocationSelectorProps) {

    // Sort locations alphabetically
    const sortedLocations = [...locations].sort((a, b) => a.localeCompare(b));

    return (
        <div className={className}>
            <Select
                value={selectedLocation || "all"}
                onValueChange={(value) => onSelect(value === "all" ? null : value)}
            >
                <SelectTrigger className="w-[200px] h-8 bg-transparent border-input text-foreground">
                    <div className="flex items-center gap-2">
                        <MapPin className="h-4 w-4 text-muted-foreground" />
                        <SelectValue placeholder="All Locations" />
                    </div>
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="all">All Locations</SelectItem>
                    {sortedLocations.map((loc) => (
                        <SelectItem key={loc} value={loc}>
                            {loc}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
        </div>
    );
}
