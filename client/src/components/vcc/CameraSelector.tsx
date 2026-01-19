import * as React from "react"
import { ChevronDown, Camera, Check } from "lucide-react"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"

export interface CameraOption {
    id: string
    name: string
}

interface CameraSelectorProps {
    cameras: CameraOption[]
    selectedCamera: string | null // null means "All Cameras"
    onSelect: (cameraId: string | null) => void
    loading?: boolean
    className?: string
}

export function CameraSelector({
    cameras,
    selectedCamera,
    onSelect,
    loading = false,
    className
}: CameraSelectorProps) {
    const [open, setOpen] = React.useState(false)

    const selectedCameraName = selectedCamera
        ? (cameras.find(c => c.id === selectedCamera)?.name || selectedCamera).replace(/^Camera\s+/i, "")
        : "All Cameras"

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    className={cn(
                        "justify-between text-left font-normal h-8 px-3 min-w-[160px]",
                        className
                    )}
                    disabled={loading}
                >
                    <div className="flex items-center gap-2 truncate">
                        <Camera className="h-4 w-4 text-muted-foreground" />
                        <span className="text-xs truncate">{selectedCameraName}</span>
                    </div>
                    <ChevronDown className="ml-2 h-3 w-3 opacity-50 flex-shrink-0" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[240px] p-0" align="start">
                <div className="max-h-[300px] overflow-y-auto">
                    {/* All Cameras option */}
                    <button
                        type="button"
                        onClick={() => {
                            onSelect(null)
                            setOpen(false)
                        }}
                        className={cn(
                            "w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent transition-colors",
                            selectedCamera === null && "bg-accent"
                        )}
                    >
                        <Check className={cn("h-4 w-4", selectedCamera === null ? "opacity-100" : "opacity-0")} />
                        <span className="font-medium">All Cameras</span>
                    </button>

                    {/* Divider */}
                    <div className="border-t my-1" />

                    {/* Camera list */}
                    {cameras.length === 0 ? (
                        <div className="px-3 py-2 text-sm text-muted-foreground">
                            No cameras available
                        </div>
                    ) : (
                        cameras.map((camera) => (
                            <button
                                key={camera.id}
                                type="button"
                                onClick={() => {
                                    onSelect(camera.id)
                                    setOpen(false)
                                }}
                                className={cn(
                                    "w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent transition-colors",
                                    selectedCamera === camera.id && "bg-accent"
                                )}
                            >
                                <Check className={cn("h-4 w-4", selectedCamera === camera.id ? "opacity-100" : "opacity-0")} />
                                <span className="truncate">{(camera.name || camera.id).replace(/^Camera\s+/i, "")}</span>
                            </button>
                        ))
                    )}
                </div>
            </PopoverContent>
        </Popover>
    )
}
