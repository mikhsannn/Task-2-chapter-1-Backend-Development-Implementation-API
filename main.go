package main

/*
sebelumnya saya izin memberi tahu alasan mengapa saya meletakkan
file nya secara keseluruhan di mainnya, sebelumnya saya meletakkanya
secar terpisah, yaitu database,minio,models,handlers nya itu terpisah, namun
pada saat di run itu file nya tidak bisa diimport ke dalam import file
main.go nya maka dai itu saya coba menggabungkan secara keseluruhannya
menjadi satu kesatuan agar dapat di run dan tidak error, walaupun berdampak
codingan sedikit sulit untuk membacanya...
MOHON MAAF APABILA SUSAH MEMBACA CODINGAN SAYA!
*/
import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Muat file .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Gagal memuat file .env:", err)
	}

	// Terhubung ke MongoDB
	dbClient, err := ConnectDatabase()
	if err != nil {
		log.Fatal("Gagal terhubung ke database:", err)
	}
	defer dbClient.Disconnect(context.TODO())

	// Inisialisasi MinIO client
	//minioClient, err := InitializeMinioClient()
	// if err != nil {
	//	log.Fatal("Gagal menginisialisasi MinIO client:", err)
	//}

	// Buat router Gin baru
	router := gin.Default()

	// Tentukan rute API di sini
	router.GET("/products", GetProductsHandler)
	router.GET("/products/:id", GetProductDetailHandler)
	router.POST("/products", CreateProductHandler)
	router.PUT("/products/:id", UpdateProductHandler)
	router.DELETE("/products/:id", DeleteProductHandler)

	// Jalankan server
	port := os.Getenv("SERVER_PORT")
	log.Printf("Server berjalan di port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

type Product struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Category string  `json:"category"`
}

type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username string             `bson:"username" json:"username"`
	Password string             `bson:"password" json:"password"`
	// Tambahkan field lain jika diperlukan
}

type AuthToken struct {
	Token string `json:"token"`
}

func ConnectDatabase() (*mongo.Client, error) {
	// Baca variabel lingkungan
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	username := os.Getenv("DB_USERNAME")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Buat URI koneksi MongoDB
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%s/%s", username, password, host, port, dbName)

	// Buat opsi koneksi MongoDB
	clientOptions := options.Client().ApplyURI(uri)

	// Buat klien MongoDB baru
	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		return nil, err
	}

	// Buat konteks dengan batas waktu
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Terhubung ke MongoDB
	err = client.Connect(ctx)
	if err != nil {
		return nil, err
	}

	log.Println("Terhubung ke MongoDB")

	return client, nil
}

func InitializeMinioClient() (*minio.Client, error) {
	// Baca variabel lingkungan
	host := os.Getenv("MINIO_HOST")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")

	// Buat MinIO client
	minioClient, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false, // Koneksi non-HTTPS
	})
	if err != nil {
		return nil, err
	}

	log.Println("MinIO client diinisialisasi")

	return minioClient, nil
}

func UploadFileToMinio(minioClient *minio.Client, file *multipart.FileHeader) (string, error) {
	// Baca variabel lingkungan
	bucketName := os.Getenv("MINIO_BUCKET")

	// Buka file yang akan diunggah
	fileData, err := file.Open()
	if err != nil {
		return "", err
	}
	defer fileData.Close()

	// Generate object key unik
	objectName := fmt.Sprintf("%s/%s", bucketName, file.Filename)

	// Unggah file ke MinIO
	_, err = minioClient.PutObject(context.TODO(), bucketName, objectName, fileData, file.Size, minio.PutObjectOptions{
		ContentType: file.Header.Get("Content-Type"),
	})
	if err != nil {
		return "", err
	}

	// Kembalikan URL file yang diunggah
	fileURL := fmt.Sprintf("%s/%s", os.Getenv("MINIO_HOST"), objectName)
	return fileURL, nil
}

func DeleteFileFromMinio(minioClient *minio.Client, objectName string) error {
	// Baca variabel lingkungan
	bucketName := os.Getenv("MINIO_BUCKET")

	// Hapus file dari MinIO
	err := minioClient.RemoveObject(context.TODO(), bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

// GetProductsHandler menghandle API untuk mendapatkan daftar produk
func GetProductsHandler(c *gin.Context) {
	// Dapatkan klien MongoDB dari konteks
	dbClient := c.MustGet("dbClient").(*mongo.Client)

	// Akses koleksi yang berisi produk
	collection := dbClient.Database("mydatabase").Collection("products")

	// Tentukan parameter query, jika ada
	filter := bson.M{} // Contoh: bson.M{"category": "elektronik"}

	// Eksekusi query dan ambil produk-produk
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil produk"})
		return
	}
	defer cursor.Close(context.Background())

	// Iterasi melalui cursor dan bangun daftar produk
	var products []Product
	for cursor.Next(context.Background()) {
		var product Product
		if err := cursor.Decode(&product); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mendecode produk"})
			return
		}
		products = append(products, product)
	}
	if err := cursor.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengiterasi produk"})
		return
	}
	c.JSON(http.StatusOK, products)
}

func generateProductID() string {
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	return fmt.Sprintf("P%d", timestamp)
}

// GetProductDetailHandler menghandle API untuk mendapatkan detail produk
func GetProductDetailHandler(c *gin.Context) {
	// Dapatkan klien MongoDB dari konteks
	dbClient := c.MustGet("dbClient").(*mongo.Client)

	// Akses koleksi yang berisi produk
	collection := dbClient.Database("mydatabase").Collection("products")

	// Dapatkan ID produk dari parameter URL
	productID := c.Param("id")

	// Tentukan query untuk mencari produk berdasarkan ID-nya
	filter := bson.M{"_id": productID}

	// Temukan produk dalam database
	var product Product
	if err := collection.FindOne(context.Background(), filter).Decode(&product); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Produk tidak ditemukan"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil produk"})
		}
		return
	}

	// Kembalikan detail produk sebagai respons JSON
	c.JSON(http.StatusOK, product)
}

// CreateProductHandler menghandle API untuk membuat produk baru
func CreateProductHandler(c *gin.Context) {
	// Dapatkan klien MongoDB dari konteks
	dbClient := c.MustGet("dbClient").(*mongo.Client)

	// Akses koleksi yang berisi produk
	collection := dbClient.Database("mydatabase").Collection("products")

	// Bind data JSON ke struct produk
	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate ID produk baru
	product.ID = generateProductID()

	// Masukkan produk ke dalam database
	_, err := collection.InsertOne(context.Background(), product)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat produk"})
		return
	}

	// Kembalikan produk yang dibuat sebagai respons JSON
	c.JSON(http.StatusOK, product)
}

// UpdateProductHandler menghandle API untuk memperbarui produk yang ada
func UpdateProductHandler(c *gin.Context) {
	// Dapatkan klien MongoDB dari konteks
	dbClient := c.MustGet("dbClient").(*mongo.Client)

	// Akses koleksi yang berisi produk
	collection := dbClient.Database("mydatabase").Collection("products")

	// Dapatkan ID produk dari parameter URL
	productID := c.Param("id")

	// Tentukan query untuk mencari produk berdasarkan ID-nya
	filter := bson.M{"_id": productID}

	// Bind data JSON ke struct produk yang diperbarui
	var updatedProduct Product
	if err := c.ShouldBindJSON(&updatedProduct); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Perbarui produk di dalam database
	update := bson.M{"$set": updatedProduct}
	_, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui produk"})
		return
	}

	// Atur ID produk yang diperbarui
	updatedProduct.ID = productID

	// Kembalikan produk yang diperbarui sebagai respons JSON
	c.JSON(http.StatusOK, updatedProduct)
}

// DeleteProductHandler handles the API to delete a product
func DeleteProductHandler(c *gin.Context) {
	// Get the MongoDB client from the context
	dbClient := c.MustGet("dbClient").(*mongo.Client)

	// Access the collection containing the products
	collection := dbClient.Database("mydatabase").Collection("products")

	// Get the product ID from the URL parameter
	productID := c.Param("id")

	// Define your query to find the product by its ID
	filter := bson.M{"_id": productID}

	// Delete the product from the database
	_, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus produk"})
		return
	}

	// Return a success message
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Produk %s telah dihapus", productID)})
}
