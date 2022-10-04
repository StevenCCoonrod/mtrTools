package sqlDataAccessor

import (
	"context"
	"database/sql"
	"fmt"
	"mtrTools/dataObjects"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
)

// Localhost variants
// var server = "localhost"
// var server = "localhost\\MSSQLSERVER02"

// DEV SQL
//var server = "ec2-54-204-1-34.compute-1.amazonaws.com"

// Syncmonitor QA
var server = "dashboard-qa-20190305.cunftndptrif.us-east-1.rds.amazonaws.com"

// SQL Server
// var port = 1433
// MySql
var port = 3306

var user = ""
var password = ""
var database = "NetopsToolsDB"

var db *sql.DB

//================== INSERT STATEMENTS ===================\\

// Inserts a Syncbox into the DB
func InsertSyncbox(syncbox string) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err error
	db := getDBConnection()

	err1 := db.PingContext(ctx)
	if err1 != nil {
		fmt.Println(err.Error())
	}

	dataReturned, err := db.QueryContext(ctx, "call sp_InsertSyncbox(?)", syncbox)
	if err != nil {
		fmt.Println(err)
	}
	dataReturned.Close()
	db.Close()
}

// Inserts an SSH MtrReport into the DB
func InsertMtrReports(mtrReports []dataObjects.MtrReport) int {
	reportsInserted := 0
	db := getDBConnection()
	for _, report := range mtrReports {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		defer db.Close()
		//Insert the Report
		var reportID int
		if report.SyncboxID != "" && report.DataCenter != "" {
			rowReturned := db.QueryRowContext(ctx, "call sp_InsertMtrReport(?,?,?)", report.SyncboxID, report.StartTime, report.DataCenter)
			if rowReturned.Err() == nil && rowReturned != nil {
				err := rowReturned.Scan(&reportID)
				if err != nil {

					if strings.Contains(err.Error(), "no rows") {
						//Report already exists
					} else {
						fmt.Println("Error inserting report. ("+fmt.Sprint(reportID)+") \n", report.PrintReport(), "\n", err.Error())
					}

				} else if reportID != 0 {
					//Insert hops for report
					var successfulHopInsertion bool
					for _, h := range report.Hops {

						_, err := db.ExecContext(ctx, "call sp_InsertMtrHop(?,?,?,?,?,?,?,?,?,?)", reportID, h.HopNumber, h.Hostname,
							h.PacketLoss, h.PacketsSent, h.LastPing, h.AveragePing, h.BestPing, h.WorstPing, h.StdDev)

						if err != nil {
							fmt.Println("Error inserting hop", h.HopNumber, "for", report.ReportID, "\n", err.Error())
							successfulHopInsertion = false
							//remove report from DB?
							break
						} else {
							successfulHopInsertion = true
						}
					}
					if successfulHopInsertion {
						reportsInserted += 1
					}
				}
			}
		}
	}

	return reportsInserted
}

//================== SELECT STATEMENTS ===================\\

// Selects all Syncboxes in the DB
func SelectAllSyncboxes() []string {

	var syncboxList []string
	var err error

	fmt.Println("Connecting...")
	db := getDBConnection()
	ctx := context.Background()

	err1 := db.PingContext(ctx)
	if err1 != nil {
		fmt.Println(err.Error())
	}

	dataReturned, err := db.QueryContext(ctx, "call sp_SelectAllSyncboxes()")
	if err != nil {
		fmt.Println(err)
	} else {
		for dataReturned.Next() {
			var syncbox string
			err := dataReturned.Scan(&syncbox)
			if err != nil {
				fmt.Println(err)
			}
			syncboxList = append(syncboxList, syncbox)
		}
		dataReturned.Close()
	}
	db.Close()
	return syncboxList
}

// Takes a batch of MtrReports, Selects them from the DB, and parses them back into MtrReports
func SelectMtrReportsByID(reports []dataObjects.MtrReport) []dataObjects.MtrReport {

	var dataReturned *sql.Rows
	var reportsReturned []dataObjects.MtrReport
	var err error
	db := getDBConnection()
	ctx := context.Background()

	err1 := db.PingContext(ctx)
	if err1 != nil {
		fmt.Println(err.Error())
	}
	for _, r := range reports {

		dataReturned, err = db.QueryContext(ctx, "call sp_SelectMtrReportByID(?)", r.ReportID)
		if err != nil {
			fmt.Println("Error selecting mtr report. ", err.Error())
		} else {
			parsedReport := parseSqlSingleReportDataIntoReport(dataReturned)
			dataReturned.Close()
			reportsReturned = append(reportsReturned, parsedReport)
		}

	}
	db.Close()
	return reportsReturned
}

// Returns all DB Reports for a specified syncbox, between two datetimes
func SelectMtrReports_BySyncbox_Timeframe(syncbox string, startTime time.Time, endTime time.Time) []dataObjects.MtrReport {

	var err error
	db := getDBConnection()
	ctx := context.Background()

	err1 := db.PingContext(ctx)
	if err1 != nil {
		fmt.Println(err1.Error())
		return nil
	}

	var dataReturned *sql.Rows
	var reportsReturned []dataObjects.MtrReport
	defer db.Close()
	dataReturned, err = db.QueryContext(ctx, "call sp_SelectMtrs_BySyncbox_WithinRange(?,?,?)", syncbox, startTime, endTime)
	if err != nil {
		fmt.Println("Error selecting mtr report. ", err.Error())
	}

	reportsReturned = append(reportsReturned, parseSqlMultipleReportDataIntoReports(dataReturned)...)

	dataReturned.Close()
	return reportsReturned
}

// Returns all DB Reports for a specified syncbox, between two datetimes, targeting a specified data center
func SelectMtrReports_BySyncbox_DCAndTimeframe(syncbox string, startTime time.Time, endTime time.Time, datacenter string) []dataObjects.MtrReport {

	var err error
	db := getDBConnection()
	ctx := context.Background()

	err1 := db.PingContext(ctx)
	if err1 != nil {
		fmt.Println(err.Error())
	}

	var dataReturned *sql.Rows
	var reportsReturned []dataObjects.MtrReport
	defer db.Close()
	dataReturned, err = db.QueryContext(ctx, "call sp_SelectMtrs_BySyncbox_DCAndTimeframe(?,?,?,?)", syncbox, startTime, endTime, strings.ToLower(datacenter))
	if err != nil {
		fmt.Println("Error selecting mtr report. ", err.Error())
	}

	reportsReturned = append(reportsReturned, parseSqlMultipleReportDataIntoReports(dataReturned)...)

	dataReturned.Close()

	return reportsReturned
}

// Returns all DB Reports for a specified syncbox, between two datetimes
func SelectMtrReports_ByHostname(hostname string) []dataObjects.MtrReport {
	var err error
	db := getDBConnection()
	ctx := context.Background()

	err1 := db.PingContext(ctx)
	if err1 != nil {
		fmt.Println(err.Error())
	}

	var dataReturned *sql.Rows
	var reportsReturned []dataObjects.MtrReport
	defer db.Close()
	dataReturned, err = db.QueryContext(ctx, "call sp_SelectMtrs_ByHostname(?)", hostname)
	if err != nil {
		fmt.Println("Error selecting reports by host name. ", err.Error())
	}

	reportsReturned = append(reportsReturned, parseSqlMultipleReportDataIntoReports(dataReturned)...)

	dataReturned.Close()

	return reportsReturned
}

//======================= HELPER METHODS =======================\\

// Parses an individual report from the DB into an MtrReport
func parseSqlSingleReportDataIntoReport(sqlRowData *sql.Rows) dataObjects.MtrReport {

	var syncboxID, dataCenter, hostName *string
	var startTime *time.Time
	var reportID, hopID, hopNumber, packetsSent *int
	var packetLoss, last, avg, best, worst, stdDev *float32
	var report dataObjects.MtrReport

	for sqlRowData.Next() {
		if err := sqlRowData.Scan(&reportID, &syncboxID, &startTime, &dataCenter, &hopID, &hopNumber,
			&hostName, &packetLoss, &packetsSent, &last, &avg, &best, &worst, &stdDev); err != nil {
			fmt.Println(err.Error())
		} else {
			if *reportID != report.ReportID {

				report = dataObjects.MtrReport{ReportID: *reportID, SyncboxID: *syncboxID, StartTime: *startTime, DataCenter: *dataCenter}
				hop := dataObjects.MtrHop{HopID: *hopID, HopNumber: *hopNumber, Hostname: *hostName,
					PacketLoss: *packetLoss, PacketsSent: *packetsSent, LastPing: *last, AveragePing: *avg,
					BestPing: *best, WorstPing: *worst, StdDev: *stdDev}
				report.Hops = append(report.Hops, hop)
			} else {
				hop := dataObjects.MtrHop{HopID: *hopID, HopNumber: *hopNumber, Hostname: *hostName,
					PacketLoss: *packetLoss, PacketsSent: *packetsSent, LastPing: *last, AveragePing: *avg,
					BestPing: *best, WorstPing: *worst, StdDev: *stdDev}

				report.Hops = append(report.Hops, hop)

			}
		}

	}

	return report
}

// Parses multiple reports from the DB into MtrReports
func parseSqlMultipleReportDataIntoReports(sqlRowData *sql.Rows) []dataObjects.MtrReport {
	var reports []dataObjects.MtrReport
	var syncboxID, dataCenter, hostName *string
	var startTime *time.Time
	var reportID, hopID, hopNumber, packetsSent *int
	var packetLoss, last, avg, best, worst, stdDev *float32
	var report dataObjects.MtrReport
	for sqlRowData.Next() {
		if err := sqlRowData.Scan(&reportID, &syncboxID, &startTime, &dataCenter, &hopID, &hopNumber,
			&hostName, &packetLoss, &packetsSent, &last, &avg, &best, &worst, &stdDev); err != nil {
			fmt.Println(err.Error())
		} else {
			if *reportID != report.ReportID {
				if report.ReportID > 0 {
					reports = append(reports, report)
				}

				report = dataObjects.MtrReport{ReportID: *reportID, SyncboxID: *syncboxID, StartTime: *startTime, DataCenter: *dataCenter}
				hop := dataObjects.MtrHop{HopID: *hopID, HopNumber: *hopNumber, Hostname: *hostName,
					PacketLoss: *packetLoss, PacketsSent: *packetsSent, LastPing: *last, AveragePing: *avg,
					BestPing: *best, WorstPing: *worst, StdDev: *stdDev}
				report.Hops = append(report.Hops, hop)
			} else {
				hop := dataObjects.MtrHop{HopID: *hopID, HopNumber: *hopNumber, Hostname: *hostName,
					PacketLoss: *packetLoss, PacketsSent: *packetsSent, LastPing: *last, AveragePing: *avg,
					BestPing: *best, WorstPing: *worst, StdDev: *stdDev}

				report.Hops = append(report.Hops, hop)
			}
		}
	}

	return reports
}

// Establishes DB connection and context
func getDBConnection() *sql.DB {
	var err error
	// SQL Server
	// connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;database=%s;",
	// 	server, user, password, port, database)
	// MySQL
	connString := fmt.Sprintf("%s:%s@(%s:%d)/%s", user, password, server, port, database)

	db, err = sql.Open("mysql", connString)
	if err != nil {
		fmt.Println("Error creating connection pool. \n" + err.Error())
	}
	return db
}
