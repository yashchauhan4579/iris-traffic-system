import { Document, Page, Text, View, StyleSheet, Font } from '@react-pdf/renderer';
import { type VCCStats, type VCCDeviceStats } from '@/lib/api';
import { format } from 'date-fns';

// Create styles
const styles = StyleSheet.create({
    page: {
        padding: 30,
        backgroundColor: '#ffffff',
        fontFamily: 'Helvetica',
    },
    header: {
        marginBottom: 20,
        borderBottomWidth: 1,
        borderBottomColor: '#cccccc',
        paddingBottom: 10,
    },
    title: {
        fontSize: 24,
        marginBottom: 5,
        color: '#1a1a1a',
        textAlign: 'center',
    },
    subtitle: {
        fontSize: 10,
        color: '#666666',
        marginTop: 5,
        textAlign: 'center',
    },
    section: {
        marginBottom: 15,
    },
    sectionTitle: {
        fontSize: 14,
        fontWeight: 'bold',
        marginBottom: 10,
        color: '#333333',
        backgroundColor: '#f0f0f0',
        padding: 5,
        textAlign: 'center',
    },
    row: {
        flexDirection: 'row',
        justifyContent: 'space-around',
        marginBottom: 5,
    },
    statBox: {
        width: '30%',
        padding: 10,
        backgroundColor: '#f8f9fa',
        borderRadius: 4,
        borderWidth: 1,
        borderColor: '#e9ecef',
        alignItems: 'center',
    },
    statLabel: {
        fontSize: 9,
        color: '#666666',
        marginBottom: 3,
        textAlign: 'center',
    },
    statValue: {
        fontSize: 16,
        color: '#000000',
        fontWeight: 'bold',
        textAlign: 'center',
    },
    table: {
        display: 'flex',
        width: '90%',
        alignSelf: 'center',
        borderStyle: 'solid',
        borderWidth: 1,
        borderRightWidth: 0,
        borderBottomWidth: 0,
        borderColor: '#bfbfbf',
    },
    tableRow: {
        flexDirection: 'row',
    },
    tableHeader: {
        backgroundColor: '#f8f9fa',
        fontWeight: 'bold',
    },
    tableCol: {
        width: '25%',
        borderStyle: 'solid',
        borderWidth: 1,
        borderLeftWidth: 0,
        borderTopWidth: 0,
        borderColor: '#bfbfbf',
    },
    tableCell: {
        margin: 5,
        fontSize: 9,
        textAlign: 'center',
    },
    tableCellRight: {
        margin: 5,
        fontSize: 9,
        textAlign: 'center',
    },
    tableCellHeader: {
        margin: 5,
        fontSize: 9,
        fontWeight: 'bold',
        textAlign: 'center',
    },
    tableCellHeaderRight: {
        margin: 5,
        fontSize: 9,
        fontWeight: 'bold',
        textAlign: 'center',
    },
    footer: {
        position: 'absolute',
        bottom: 30,
        left: 30,
        right: 30,
        textAlign: 'center',
        color: '#999999',
        fontSize: 8,
        borderTopWidth: 1,
        borderTopColor: '#cccccc',
        paddingTop: 10,
    },
});

interface VCCReportPDFProps {
    stats: VCCStats | VCCDeviceStats | null;
    startDate: Date;
    endDate: Date;
    selectedCameraName?: string;
}

export function VCCReportPDF({ stats, startDate, endDate, selectedCameraName }: VCCReportPDFProps) {
    if (!stats) return <Document><Page><Text>No data available</Text></Page></Document>;

    const safeStats = stats as any;
    const isPerDevice = 'deviceId' in stats;

    const reportTitle = isPerDevice
        ? `Traffic Analysis Report: ${safeStats.deviceName || safeStats.deviceId}`
        : `Traffic Analysis Report: All Cameras`;

    const dateRangeStr = `${format(startDate, 'MMM d, yyyy')} - ${format(endDate, 'MMM d, yyyy')}`;
    const generatedAt = format(new Date(), 'MMM d, yyyy HH:mm');

    // Prepare table data
    // Standard types matching Dashboard (excluding TRUCK as requested)
    const displayTypes = ['2W', '4W', 'AUTO', 'BUS', 'HMV'];

    const topDevices = !isPerDevice && safeStats.byDevice
        ? safeStats.byDevice
        : [];

    return (
        <Document>
            <Page size="A4" style={styles.page}>
                {/* Header */}
                <View style={styles.header}>
                    <Text style={styles.title}>{reportTitle}</Text>
                    <Text style={styles.subtitle}>Report Period: {dateRangeStr}</Text>
                    <Text style={styles.subtitle}>Generated on: {generatedAt}</Text>
                </View>

                {/* Summary Stats */}
                <View style={styles.row}>
                    <View style={styles.statBox}>
                        <Text style={styles.statLabel}>Total Detections</Text>
                        <Text style={styles.statValue}>{safeStats.totalDetections.toLocaleString()}</Text>
                    </View>
                    <View style={styles.statBox}>
                        <Text style={styles.statLabel}>Avg Per Minute</Text>
                        <Text style={styles.statValue}>{(safeStats.averagePerHour / 60).toFixed(1)}</Text>
                    </View>
                    <View style={styles.statBox}>
                        <Text style={styles.statLabel}>Avg Per Hour</Text>
                        <Text style={styles.statValue}>{safeStats.averagePerHour.toFixed(1)}</Text>
                    </View>
                </View>

                <View style={[styles.row, { marginTop: 10, marginBottom: 20 }]}>
                    <View style={styles.statBox}>
                        <Text style={styles.statLabel}>Peak Hour</Text>
                        <Text style={styles.statValue}>{safeStats.peakHour}:00</Text>
                    </View>
                    <View style={styles.statBox}>
                        <Text style={styles.statLabel}>Peak Day</Text>
                        <Text style={styles.statValue}>{safeStats.peakDay}</Text>
                    </View>
                    <View style={styles.statBox}>
                        <Text style={styles.statLabel}>Total Classes</Text>
                        <Text style={styles.statValue}>{displayTypes.length}</Text>
                    </View>
                </View>

                {/* Vehicle Classification Table */}
                <View style={styles.section}>
                    <Text style={styles.sectionTitle}>Vehicle Classification Breakdown</Text>
                    <View style={styles.table}>
                        <View style={[styles.tableRow, styles.tableHeader]}>
                            <View style={[styles.tableCol, { width: '40%' }]}><Text style={styles.tableCellHeader}>Vehicle Type</Text></View>
                            <View style={[styles.tableCol, { width: '30%' }]}><Text style={styles.tableCellHeaderRight}>Count</Text></View>
                            <View style={[styles.tableCol, { width: '30%' }]}><Text style={styles.tableCellHeaderRight}>Percentage</Text></View>
                        </View>
                        {displayTypes.map((type) => {
                            const count = Number(safeStats.byVehicleType?.[type]) || 0;
                            const percentage = safeStats.totalDetections > 0
                                ? ((count / safeStats.totalDetections) * 100).toFixed(1) + '%'
                                : '0%';

                            return (
                                <View key={type} style={styles.tableRow}>
                                    <View style={[styles.tableCol, { width: '40%' }]}><Text style={styles.tableCell}>{type}</Text></View>
                                    <View style={[styles.tableCol, { width: '30%' }]}><Text style={styles.tableCellRight}>{count.toLocaleString()}</Text></View>
                                    <View style={[styles.tableCol, { width: '30%' }]}><Text style={styles.tableCellRight}>{percentage}</Text></View>
                                </View>
                            );
                        })}
                    </View>
                </View>

                {/* Active Devices Table (Only for All Cameras view) */}
                {topDevices.length > 0 && (
                    <View style={styles.section}>
                        <Text style={styles.sectionTitle} break>Active Devices</Text>
                        <View style={styles.table}>
                            {/* Header Row - Fixed for pagination */}
                            <View style={[styles.tableRow, styles.tableHeader]} fixed>
                                <View style={[styles.tableCol, { width: '25%' }]}><Text style={styles.tableCellHeader}>Device Name</Text></View>
                                <View style={[styles.tableCol, { width: '15%' }]}><Text style={styles.tableCellHeaderRight}>Total</Text></View>
                                {displayTypes.map(type => (
                                    <View key={type} style={[styles.tableCol, { width: '12%' }]}>
                                        <Text style={styles.tableCellHeaderRight}>{type}</Text>
                                    </View>
                                ))}
                            </View>
                            {/* Data Rows */}
                            {topDevices.map((device: any) => (
                                <View key={device.deviceId} style={styles.tableRow}>
                                    <View style={[styles.tableCol, { width: '25%' }]}>
                                        <Text style={styles.tableCell}>{device.deviceName || device.deviceId}</Text>
                                    </View>
                                    <View style={[styles.tableCol, { width: '15%' }]}>
                                        <Text style={styles.tableCellRight}>
                                            {(device.totalDetections || 0).toLocaleString()}
                                        </Text>
                                    </View>
                                    {displayTypes.map(type => {
                                        // Handle both new structure (byType) and legacy (if cached)
                                        const typeCount = device.byType ? (device.byType[type] || 0) : 0;
                                        return (
                                            <View key={type} style={[styles.tableCol, { width: '12%' }]}>
                                                <Text style={styles.tableCellRight}>{Number(typeCount).toLocaleString()}</Text>
                                            </View>
                                        );
                                    })}
                                </View>
                            ))}
                        </View>
                    </View>
                )}

                <Text style={styles.footer}>
                    Generated by Iris VCC System - {generatedAt}
                </Text>
            </Page>
        </Document>
    );
}
